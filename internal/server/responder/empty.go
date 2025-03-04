package responder

import (
	"fmt"
	"net/http"
)

type Empty struct {
	message string
}

func NewEmptyf(format string, args ...any) Responder {
	return &Empty{
		message: fmt.Sprintf(format, args...),
	}
}

func (empty *Empty) Respond(_ http.ResponseWriter, _ *http.Request) {
	// do nothing
}

func (empty *Empty) Message() string {
	return empty.message
}
