package responder

import "net/http"

type Responder interface {
	Respond(writer http.ResponseWriter, request *http.Request)
	Message() string
}
