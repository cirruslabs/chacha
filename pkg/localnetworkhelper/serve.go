//go:build darwin

package localnetworkhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"syscall"
)

// Serve implements a privileged component of the macOS "Local Network" permission helper.
//
// It listens for net.Dial requests, performs the dialing and sends the results back.
func Serve(fd int) error {
	// Convert our end of the socketpair(2) to a *unix.Conn
	file := os.NewFile(uintptr(fd), "")

	conn, err := net.FileListener(file)
	if err != nil {
		return err
	}

	// We can safely close the fd now as it was dup(2)'ed by the net.FileConn
	if err := file.Close(); err != nil {
		return err
	}

	unixListener, ok := conn.(*net.UnixListener)
	if !ok {
		return fmt.Errorf("expected *net.UnixConn, got %T", conn)
	}

	// Serve requests
	for {
		unixConn, err := unixListener.AcceptUnix()
		if err != nil {
			return err
		}

		go handle(unixConn)
	}
}

func handle(unixConn *net.UnixConn) error {
	// Ensure that the remote is our parent
	peerPID, err := getPeerPID(unixConn)
	if err != nil {
		return fmt.Errorf("failed to retrieve peer's PID: %w, "+
			"refusing to dial", err)
	}

	if peerPID != os.Getppid() {
		return fmt.Errorf("peer's PID %d is different than our parent PID %d, "+
			"refusing to dial", peerPID, os.Getppid())
	}

	buf := make([]byte, 4096)

	n, err := unixConn.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}

	var privilegedSocketRequest PrivilegedSocketRequest

	if err := json.Unmarshal(buf[:n], &privilegedSocketRequest); err != nil {
		return err
	}

	var privilegedSocketResponse PrivilegedSocketResponse
	var oob []byte

	netConn, err := net.Dial(privilegedSocketRequest.Network, privilegedSocketRequest.Addr)
	if err != nil {
		privilegedSocketResponse.Error = err.Error()
	} else {
		defer netConn.Close()

		var syscallConn syscall.RawConn

		switch typedNetConn := netConn.(type) {
		case *net.TCPConn:
			syscallConn, err = typedNetConn.SyscallConn()
		case *net.UDPConn:
			syscallConn, err = typedNetConn.SyscallConn()
		default:
			err = fmt.Errorf("unsupported net.Conn type: %T", netConn)
		}
		if err != nil {
			privilegedSocketResponse.Error = err.Error()
		}

		if syscallConn != nil {
			if err := syscallConn.Control(func(fd uintptr) {
				oob = unix.UnixRights(int(fd))
			}); err != nil {
				privilegedSocketResponse.Error = err.Error()
			}
		}
	}

	privilegedSocketResponseJSONBytes, err := json.Marshal(privilegedSocketResponse)
	if err != nil {
		return err
	}

	_, _, err = unixConn.WriteMsgUnix(privilegedSocketResponseJSONBytes, oob, nil)
	if err != nil {
		return err
	}

	return nil
}

func getPeerPID(unixConn *net.UnixConn) (int, error) {
	syscallConn, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}

	var pid int

	if err := syscallConn.Control(func(fd uintptr) {
		pid, err = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	}); err != nil {
		return 0, err
	}

	return pid, nil
}
