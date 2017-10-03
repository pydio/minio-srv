package dav

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/fatih/color"
	"github.com/pydio/services/common/service"

	"github.com/micro/cli"
	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/views"
	"golang.org/x/net/webdav"
)

type ValidUser struct {
	Hash      string
	Connexion time.Time
	Claims    auth.Claims
}

var (
	validUsers = map[string]*ValidUser{}
)

func NewDAVService(ctx *cli.Context) (micro.Service, error) {

	service := service.NewService(
		micro.Name("pydio.service.dav"),
	)

	port := 5013
	if ctx.Int("dav_port") > 0 {
		port = ctx.Int("dav_port")
	}

	go startHttpServer(service.Options().Context, port)

	return service, nil

}

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func startHttpServer(ctx context.Context, port int) {

	router := views.NewStandardRouter(false, true)
	basicAuthenticator := auth.NewBasicAuthenticator("Pydio WebDAV", time.Duration(10*time.Minute))

	fs := &FileSystem{
		Router: router,
		Debug:  true,
		mu:     sync.Mutex{},
	}

	dav := &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			switch r.Method {
			case "COPY", "MOVE":
				dst := ""
				if u, err := url.Parse(r.Header.Get("Destination")); err == nil {
					dst = u.Path
				}
				log.Logger(ctx).Debug("DAV handler", zap.String("method", color.GreenString(r.Method)), zap.String("path", r.URL.Path), zap.String("destination", dst), zap.Error(err))
			default:
				log.Logger(ctx).Debug("DAV handler", zap.String("method", color.GreenString(r.Method)), zap.String("path", r.URL.Path), zap.Error(err))
				// log.Printf("%-18s %s %s",
				// 	color.GreenString(r.Method),
				// 	r.URL.Path,
				// 	color.RedString(errorString(err)))
			}
		},
	}

	handler := basicAuthenticator.Wrap(dav)
	http.ListenAndServe(fmt.Sprintf(":%d", port), handler)

}
