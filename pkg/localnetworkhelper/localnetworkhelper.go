package localnetworkhelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

const CommandName = "localnetworkhelper"

type PrivilegedSocketRequest struct {
	Network string `json:"network"`
	Addr    string `json:"addr"`
}

type PrivilegedSocketResponse struct {
	Error string `json:"error"`
}

type LocalNetworkHelper struct {
	unixConn *net.UnixConn

	mtx sync.Mutex
}

// New starts a privileged part of the macOS "Local Network" permission
// helper as a child process and enables communication with it.
func New(ctx context.Context) (*LocalNetworkHelper, error) {
	// Create a socketpair(2) for communicating with the helper process
	socketPair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}

	// Launch our executable as a child process
	//
	// We're specifying the CommandName argument,
	// so that the child will jump to Serve()
	// and will wait for us.
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, executable, CommandName)

	cmd.ExtraFiles = []*os.File{
		os.NewFile(uintptr(socketPair[1]), ""),
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.WaitDelay = time.Second

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		if err := cmd.Wait(); !errors.Is(ctx.Err(), context.Canceled) {
			panic(err)
		}
	}()

	// Convert our end of the socketpair(2) to a *unix.Conn
	conn, err := net.FileConn(os.NewFile(uintptr(socketPair[0]), ""))
	if err != nil {
		return nil, err
	}

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("expected *net.UnixConn, got %T", conn)
	}

	return &LocalNetworkHelper{
		unixConn: unixConn,
	}, nil
}

func (localNetworkHelper *LocalNetworkHelper) PrivilegedDialContext(
	ctx context.Context,
	network string,
	addr string,
) (net.Conn, error) {
	// Prevent concurrency to avoid intermixing requests and responses
	localNetworkHelper.mtx.Lock()
	defer localNetworkHelper.mtx.Unlock()

	privilegedSocketRequest := PrivilegedSocketRequest{
		Network: network,
		Addr:    addr,
	}

	privilegedSocketRequestJSONBytes, err := json.Marshal(&privilegedSocketRequest)
	if err != nil {
		return nil, err
	}

	_, err = localNetworkHelper.unixConn.Write(privilegedSocketRequestJSONBytes)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	oob := make([]byte, 4096)

	n, oobn, _, _, err := localNetworkHelper.unixConn.ReadMsgUnix(buf, oob)
	if err != nil {
		return nil, err
	}

	var privilegedSocketResponse PrivilegedSocketResponse

	if err := json.Unmarshal(buf[:n], &privilegedSocketResponse); err != nil {
		return nil, err
	}

	if privilegedSocketResponse.Error != "" {
		return nil, fmt.Errorf("%s", privilegedSocketResponse.Error)
	}

	socketControlMessages, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return nil, err
	}

	if len(socketControlMessages) != 1 {
		return nil, fmt.Errorf("expected exactly one socket control message, got %d", len(socketControlMessages))
	}

	unixRights, err := unix.ParseUnixRights(&socketControlMessages[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse unix rights: %w", err)
	}

	if len(unixRights) != 1 {
		return nil, fmt.Errorf("expected exactly one unix right, got %d", len(unixRights))
	}

	netFile := os.NewFile(uintptr(unixRights[0]), "")

	netConn, err := net.FileConn(netFile)
	if err != nil {
		return nil, err
	}

	return netConn, nil
}
