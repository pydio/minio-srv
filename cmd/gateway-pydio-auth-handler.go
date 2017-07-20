package cmd

import (
	"net/http"
	"context"
)

const (
	PydioCtxUserKey = "X-Pydio-User"
)
// authHandler - handles all the incoming authorization headers and validates them if possible.
type pydioAuthHandler struct {
	handler http.Handler
}

// setAuthHandler to validate authorization header for the incoming request.
func setPydioAuthHandler(h http.Handler) http.Handler {
	return pydioAuthHandler{h}
}


// handler for validating incoming authorization headers.
func (a pydioAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	initial := r.Context()
	newContext := context.WithValue(initial, PydioCtxUserKey, "pydio-user")
	newRequest := r.WithContext(newContext)
	a.handler.ServeHTTP(w, newRequest)

}
