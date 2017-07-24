package cmd

import (
	"net/http"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common"
	"context"
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

	var md map[string]string
	var ok bool
	if md, ok = metadata.FromContext(r.Context()); !ok {
		md = make(map[string]string)
	}
	md[common.PYDIO_CONTEXT_USER_KEY] = "gateway-pydio-user"
	newContext := metadata.NewContext(r.Context(), md)
	// Add it also as value for easier use in the gateway, but this will not be transmitted
	newContext = context.WithValue(newContext, common.PYDIO_CONTEXT_USER_KEY, "gateway-pydio-user")
	newRequest := r.WithContext(newContext)
	a.handler.ServeHTTP(w, newRequest)

}
