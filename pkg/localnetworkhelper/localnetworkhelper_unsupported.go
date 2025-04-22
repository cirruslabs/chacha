//go:build !unix

package localnetworkhelper

import (
	"context"
	"net"
)

type LocalNetworkHelper struct{}

func New(ctx context.Context) (*LocalNetworkHelper, error) {
	return nil, ErrUnsupportedPlatform
}

func (localNetworkHelper *LocalNetworkHelper) PrivilegedDialContext(
	_ context.Context,
	_ string,
	_ string,
) (net.Conn, error) {
	return nil, ErrUnsupportedPlatform
}

func (localNetworkHelper *LocalNetworkHelper) Close() error {
	return ErrUnsupportedPlatform
}
