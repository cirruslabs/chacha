package singleconnlistener_test

import (
	"github.com/cirruslabs/chacha/internal/server/singleconnlistener"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func TestSingleConnListener(t *testing.T) {
	// Create a dummy connection
	dummyConn, err := net.Dial("udp", "127.0.0.1:1234")
	require.NoError(t, err)

	// Wrap it in a SingleConnListener
	singleConnListener := singleconnlistener.New(dummyConn)

	// First Accept() should return the wrapped connection
	returnedConn, err := singleConnListener.Accept()
	require.NoError(t, err)
	require.Equal(t, dummyConn, returnedConn)

	// Second Accept() should return an error
	returnedConn, err = singleConnListener.Accept()
	require.ErrorIs(t, err, net.ErrClosed)
	require.Nil(t, returnedConn)
}
