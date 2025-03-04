package singleconnlistener

import (
	"net"
	"sync/atomic"
)

type SingleConnListener struct {
	conn net.Conn
	done atomic.Bool
}

func New(conn net.Conn) *SingleConnListener {
	return &SingleConnListener{
		conn: conn,
	}
}

func (singleConnListener *SingleConnListener) Accept() (net.Conn, error) {
	if singleConnListener.done.CompareAndSwap(false, true) {
		return singleConnListener.conn, nil
	}

	return nil, net.ErrClosed
}

func (singleConnListener *SingleConnListener) Close() error {
	return nil
}

func (singleConnListener *SingleConnListener) Addr() net.Addr {
	return singleConnListener.conn.LocalAddr()
}
