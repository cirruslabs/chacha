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
	conn, err := net.FileConn(os.NewFile(uintptr(fd), ""))
	if err != nil {
		return err
	}

	// We can safely close the fd now as it was dup(2)'ed by the net.FileConn
	if err := unix.Close(fd); err != nil {
		return err
	}

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("expected *net.UnixConn, got %T", conn)
	}

	// Serve requests
	buf := make([]byte, 4096)

	for {
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
			_ = netConn.Close()

			return err
		}

		_, _, err = unixConn.WriteMsgUnix(privilegedSocketResponseJSONBytes, oob, nil)
		if err != nil {
			_ = netConn.Close()

			return err
		}

		_ = netConn.Close()
	}
}
