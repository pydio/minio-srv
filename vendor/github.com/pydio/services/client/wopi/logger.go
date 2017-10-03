package wopi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pydio/services/common/log"
)

func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		inner.ServeHTTP(w, r)

		log.Logger(r.Context()).Debug(
			fmt.Sprintf("%s %s %s %s",
				r.Method,
				r.RequestURI,
				name,
				time.Since(start),
			))
	})
}
