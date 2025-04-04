package localnetworkhelper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/samber/lo"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	CommandName = "localnetworkhelper"

	socketName = "localnetworkhelper.sock"
)

type PrivilegedSocketRequest struct {
	Network string `json:"network"`
	Addr    string `json:"addr"`
}

type PrivilegedSocketResponse struct {
	Error string `json:"error"`
}

type LocalNetworkHelper struct {
	tmpDir string
}

// New starts a privileged part of the macOS "Local Network" permission
// helper as a child process and enables communication with it.
func New(ctx context.Context, dir string, pattern string) (*LocalNetworkHelper, error) {
	socketDir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return nil, err
	}

	unixSocket, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: filepath.Join(socketDir, socketName),
	})
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

	chachaUnixFile, err := unixSocket.File()
	if err != nil {
		return nil, err
	}

	cmd.ExtraFiles = []*os.File{
		chachaUnixFile,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.WaitDelay = time.Second

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// We can safely close the unixSocket now as it was inherited by the child
	unixSocket.SetUnlinkOnClose(false)

	if err := unixSocket.Close(); err != nil {
		return nil, err
	}

	go func() {
		if err := cmd.Wait(); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			panic(err)
		}
	}()

	return &LocalNetworkHelper{
		tmpDir: socketDir,
	}, nil
}

func (localNetworkHelper *LocalNetworkHelper) PrivilegedDialContext(
	ctx context.Context,
	network string,
	addr string,
) (net.Conn, error) {
	dialer := &net.Dialer{}

	conn, err := dialer.DialContext(ctx, "unix", filepath.Join(localNetworkHelper.tmpDir, socketName))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("expected *net.UnixConn, got %T", conn)
	}

	privilegedSocketRequest := PrivilegedSocketRequest{
		Network: network,
		Addr:    addr,
	}

	privilegedSocketRequestJSONBytes, err := json.Marshal(&privilegedSocketRequest)
	if err != nil {
		return nil, err
	}

	// Send request
	writeResultCh := make(chan lo.Tuple2[int, error], 1)

	go func() {
		writeResultCh <- lo.T2(unixConn.Write(privilegedSocketRequestJSONBytes))
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-writeResultCh:
		_, err := lo.Unpack2(result)
		if err != nil {
			return nil, err
		}
	}

	buf := make([]byte, 4096)
	oob := make([]byte, 4096)

	// Receive response
	readMsgUnixResultCh := make(chan lo.Tuple5[int, int, int, *net.UnixAddr, error], 1)

	go func() {
		readMsgUnixResultCh <- lo.T5(unixConn.ReadMsgUnix(buf, oob))
	}()

	var n, oobn int

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-readMsgUnixResultCh:
		n, oobn, _, _, err = lo.Unpack5(result)
		if err != nil {
			return nil, err
		}
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

	// We can safely close the unixRights[0] now as it was dup(2)'ed by the net.FileConn
	if err := netFile.Close(); err != nil {
		return nil, err
	}

	return netConn, nil
}

func (localNetworkHelper *LocalNetworkHelper) Close() error {
	return os.RemoveAll(localNetworkHelper.tmpDir)
}
