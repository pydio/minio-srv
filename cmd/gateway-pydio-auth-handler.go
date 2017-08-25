package cmd

import (
	"net/http"
	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common"
	"context"
	"strings"
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
	var userName string

	if values, ok := r.Header[common.PYDIO_CONTEXT_USER_KEY]; ok && len(values) > 0 {
		userName = strings.Join(values, "")
		log.Printf("DETECTED USER IN REQUEST HEADERS %s\n", userName)
	} else {
		userName = "gateway-pydio-user"
	}

	md = make(map[string]string)
	md[common.PYDIO_CONTEXT_USER_KEY] = userName
	newContext := metadata.NewContext(r.Context(), md)

	// Add it as value for easier use inside the gateway, but this will not be transmitted
	newContext = context.WithValue(newContext, common.PYDIO_CONTEXT_USER_KEY, userName)
	newRequest := r.WithContext(newContext)
	a.handler.ServeHTTP(w, newRequest)

}
