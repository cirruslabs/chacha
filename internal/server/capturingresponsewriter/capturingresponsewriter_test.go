package capturingresponsewriter_test

import (
	"github.com/cirruslabs/chacha/internal/server/capturingresponsewriter"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

type dummyResponseWriter struct {
	header http.Header
}

func (writer *dummyResponseWriter) Header() http.Header {
	return writer.header
}

func (writer *dummyResponseWriter) Write(bytes []byte) (int, error) {
	return len(bytes), nil
}

func (writer *dummyResponseWriter) WriteHeader(_ int) {
	// do nothing
}

func TestCapturingResponseWriter(t *testing.T) {
	capturingResponseWriter := capturingresponsewriter.Wrap(&dummyResponseWriter{header: http.Header{}})

	require.Equal(t, 0, capturingResponseWriter.StatusCode())

	capturingResponseWriter.WriteHeader(http.StatusOK)
	require.Equal(t, http.StatusOK, capturingResponseWriter.StatusCode())

	capturingResponseWriter.WriteHeader(http.StatusTeapot)
	require.Equal(t, http.StatusTeapot, capturingResponseWriter.StatusCode())
}
