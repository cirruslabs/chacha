package server

import (
	"crypto/tls"
	"errors"
	"github.com/cirruslabs/chacha/internal/server/responder"
	"github.com/cirruslabs/chacha/internal/server/singleconnlistener"
	"net"
	"net/http"
	"time"
)

func (server *Server) handleProxyConnect(writer http.ResponseWriter, request *http.Request) responder.Responder {
	if server.tlsInterceptor == nil {
		return responder.NewCodef(http.StatusMethodNotAllowed, "no TLS interceptor is configured, "+
			"rejecting the request")
	}

	host, port, err := net.SplitHostPort(request.Host)
	if err != nil {
		return responder.NewCodef(http.StatusBadRequest, "failed to parse Host header: %v", err)
	}

	if port != "443" {
		return responder.NewCodef(http.StatusNotAcceptable, "only CONNECTs to port 443 are allowed")
	}

	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		return responder.NewCodef(http.StatusInternalServerError, "failed to hijack the connection: "+
			"writer does not implement the http.Hijacker")
	}

	netConn, bufrw, err := hijacker.Hijack()
	if err != nil {
		return responder.NewCodef(http.StatusInternalServerError, "failed to hijack the connection: "+
			"%v", err)
	}

	_, err = bufrw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	if err != nil {
		return responder.NewEmptyf("failed to write HTTP/1.1 200 Connection Established string: %v", err)
	}

	if err := bufrw.Flush(); err != nil {
		return responder.NewEmptyf("failed to flush HTTP/1.1 200 Connection Established string: %v", err)
	}

	hostCert, err := server.tlsInterceptor.GenerateCertificate(host)
	if err != nil {
		return responder.NewEmptyf("failed to generate X.509 certificate: %v", err)
	}

	//nolint:gosec // G402 yields false-positive because "By default, TLS 1.2 is currently used as the minimum".
	tlsConn := tls.Server(netConn, &tls.Config{
		Certificates: []tls.Certificate{
			*hostCert,
		},
	})

	if err := tlsConn.HandshakeContext(request.Context()); err != nil {
		return responder.NewEmptyf("failed to perofrm TLS connection handshake: %v", err)
	}

	ephemeralHTTPServer := &http.Server{
		Handler:           server,
		ReadHeaderTimeout: 30 * time.Second,
	}

	if err := ephemeralHTTPServer.Serve(singleconnlistener.New(tlsConn)); err != nil && !errors.Is(err, net.ErrClosed) {
		return responder.NewEmptyf("ephemeral HTTP server failed: %v", err)
	}

	return responder.NewEmptyf("TLS intercepted and re-routed to an ephemeral HTTP server")
}
