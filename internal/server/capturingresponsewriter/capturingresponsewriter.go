package capturingresponsewriter

import (
	"net/http"
)

type CapturingResponseWriter struct {
	statusCode int

	http.ResponseWriter
}

func Wrap(writer http.ResponseWriter) *CapturingResponseWriter {
	return &CapturingResponseWriter{
		ResponseWriter: writer,
	}
}

func (writer *CapturingResponseWriter) StatusCode() int {
	return writer.statusCode
}

// Unwrap enables interoperation with *http.ResponseController.
func (writer *CapturingResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *CapturingResponseWriter) Header() http.Header {
	return writer.ResponseWriter.Header()
}

func (writer *CapturingResponseWriter) Write(bytes []byte) (int, error) {
	return writer.ResponseWriter.Write(bytes)
}

func (writer *CapturingResponseWriter) WriteHeader(statusCode int) {
	writer.statusCode = statusCode

	writer.ResponseWriter.WriteHeader(statusCode)
}
