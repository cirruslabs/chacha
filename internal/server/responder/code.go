package responder

import (
	"fmt"
	"net/http"
)

type Code struct {
	statusCode int
	message    string
}

func NewCodef(code int, format string, args ...any) *Code {
	return &Code{
		statusCode: code,
		message:    fmt.Sprintf(format, args...),
	}
}

func (code *Code) Respond(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(code.statusCode)
}

func (code *Code) Message() string {
	return code.message
}
