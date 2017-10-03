package wopi

import (
	"net/http"

	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
)

func Auth(inner http.Handler) http.Handler {

	jwtVerifier := auth.DefaultJWTVerifier()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		if bearer := r.URL.Query().Get("access_token"); len(bearer) > 0 {

			var err error
			var claims auth.Claims
			ctx, claims, err = jwtVerifier.Verify(ctx, bearer)
			if err == nil && claims.Name != "" {
				r = r.WithContext(ctx)
				inner.ServeHTTP(w, r)
				return
			}

		}
		log.Logger(ctx).Error("Wopi API: Authentication error")
		w.WriteHeader(http.StatusUnauthorized)

	})
}
