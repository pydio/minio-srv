package auth

import (
	"net/http"
	"time"

	"github.com/micro/go-micro/metadata"
	"github.com/pydio/services/common"
	"golang.org/x/net/context"
)

func NewBasicAuthenticator(realm string, ttl time.Duration) *BasicAuthenticator {
	ba := &BasicAuthenticator{}
	ba.cache = make(map[string]*validBasicUser)
	return ba
}

type validBasicUser struct {
	Hash      string
	Connexion time.Time
	Claims    Claims
}

type BasicAuthenticator struct {
	TTL   time.Duration
	Realm string
	cache map[string]*validBasicUser
}

func (b *BasicAuthenticator) Wrap(handler http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		if user, pass, ok := r.BasicAuth(); ok {

			ctx := r.Context()

			if valid, vOk := b.cache[user]; vOk && time.Now().Sub(valid.Connexion) <= time.Duration(time.Minute*10) && valid.Hash == pass {

				var md map[string]string
				var ok bool
				if md, ok = metadata.FromContext(ctx); !ok {
					md = make(map[string]string)
				}
				md[common.PYDIO_CONTEXT_USER_KEY] = valid.Claims.Name
				ctx = metadata.NewContext(ctx, md)

				r = r.WithContext(context.WithValue(ctx, PYDIO_CONTEXT_CLAIMS_KEY, valid.Claims))

				valid.Connexion = time.Now()
				handler.ServeHTTP(w, r)
				return
			}

			jwtHelper := DefaultJWTVerifier()
			newCtx, claims, err := jwtHelper.PasswordCredentialsToken(ctx, user, pass)
			if err == nil {
				r = r.WithContext(newCtx)
				b.cache[user] = &validBasicUser{
					Hash:      pass,
					Connexion: time.Now(),
					Claims:    claims,
				}
				handler.ServeHTTP(w, r)
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="`+b.Realm+`"`)
		w.WriteHeader(401)
		w.Write([]byte("Unauthorized.\n"))

	}

}
