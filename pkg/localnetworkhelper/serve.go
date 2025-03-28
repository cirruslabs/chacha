package localnetworkhelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"syscall"
	"time"
)

// Serve implements a privileged component of the macOS "Local Network" permission helper.
//
// It listens for net.Dial requests, performs the dialing and sends the results back.
func Serve(ctx context.Context, fd int) error {
	// Convert our end of the socketpair(2) to a *unix.Conn
	conn, err := net.FileConn(os.NewFile(uintptr(fd), ""))
	if err != nil {
		return err
	}

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("expected *net.UnixConn, got %T", conn)
	}

	// Serve requests
	buf := make([]byte, 4096)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Proceed with serving
		}

		// Do not block, periodically check for context cancellation
		if err := unixConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return err
		}

		n, err := unixConn.Read(buf)
		if err != nil {
			var netError net.Error

			if errors.As(err, &netError) && netError.Timeout() {
				continue
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
