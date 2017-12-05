package cmd

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/auth"
	pydiolog "github.com/pydio/services/common/log"
)

// authHandler - handles all the incoming authorization headers and validates them if possible.
type pydioAuthHandler struct {
	handler     http.Handler
	jwtVerifier *auth.JWTVerifier
	gateway     bool
}

// setAuthHandler to validate authorization header for the incoming request.
func getPydioAuthHandlerFunc(gateway bool) HandlerFunc {
	return func(h http.Handler) http.Handler {
		return pydioAuthHandler{
			handler:     h,
			jwtVerifier: auth.DefaultJWTVerifier(),
			gateway:     gateway,
		}
	}
}


// handler for validating incoming authorization headers.
func (a pydioAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	var md map[string]string
	var userName string
	ctx := r.Context()

	jwt := r.URL.Query().Get("pydio_jwt")
	if a.gateway && len(jwt) > 0 {
		pydiolog.Logger(ctx).Debug("Found JWT in URL: replace by header and remove from URL")
		r.Header.Set("X-Pydio-Bearer", jwt)
		r.URL.RawQuery = strings.Replace(r.URL.RawQuery, "&pydio_jwt="+jwt, "", 1)
	}

	if bearer, ok := r.Header["X-Pydio-Bearer"]; ok && len(bearer) > 0 {

		rawIDToken := strings.Join(bearer, "")
		var err error
		var claims auth.Claims
		ctx, claims, err = a.jwtVerifier.Verify(ctx, rawIDToken)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		userName = claims.Name

	} else if values, ok := r.Header[common.PYDIO_CONTEXT_USER_KEY]; ok && len(values) > 0 {

		userName = strings.Join(values, "")

	} else if agent, aOk := r.Header["User-Agent"]; aOk && strings.Contains(strings.Join(agent, ""), "pydio.sync.client.s3") {

		// TODO : HOW-TO PROPERLY AUTHENTICATE SYNC CLIENT ?
		userName = "pydio.sync.client.s3"

	} else {

		if a.gateway {
			pydiolog.Logger(ctx).Error("S3 Gateway: could not find neither X-Pydio-Bearer nor X-Pydio-User in headers - access will be anonymous", zap.Any("request", r))
			a.handler.ServeHTTP(w, r)
		} else {
			pydiolog.Logger(ctx).Error("S3 DataSource: could not find neither X-Pydio-Bearer nor X-Pydio-User in headers, error 401", zap.Any("request", r))
			w.WriteHeader(http.StatusUnauthorized)
		}
		return

	}

	//pydiolog.Logger(ctx).Debug("S3 Gateway: Detected user in headers", zap.String("username", userName))
	md = make(map[string]string)
	md[common.PYDIO_CONTEXT_USER_KEY] = userName
	newContext := metadata.NewContext(ctx, md)

	// Add it as value for easier use inside the gateway, but this will not be transmitted
	newContext = context.WithValue(newContext, common.PYDIO_CONTEXT_USER_KEY, userName)
	newRequest := r.WithContext(newContext)
	a.handler.ServeHTTP(w, newRequest)

}
