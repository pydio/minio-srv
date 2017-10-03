package service

import (
	"strings"

	"go.uber.org/zap"
	"golang.org/x/net/context"

	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/server"
	api "github.com/micro/micro/api/proto"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/auth"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service/context"
)

func newJWTProvider(service micro.Service) {
	var options []micro.Option

	options = append(options, micro.WrapHandler(newJWTHandlerWrapper()))

	service.Init(options...)
}

func newJWTHandlerWrapper() server.HandlerWrapper {

	jwtVerifier := auth.DefaultJWTVerifier()

	return func(h server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			config := servicecontext.GetConfig(ctx)

			log.Logger(ctx).Info("config", zap.Any("config", config))

			allowAnon, ok := config.Get("allowAnon").(bool)
			if !ok {
				allowAnon = false
			}

			jwtValid := false

			if httpRequest, ok := (req.Request()).(*api.Request); ok {
				if val, ok1 := httpRequest.Header["Authorization"]; ok1 {
					whole := strings.Join(val.Values, "")
					rawIDToken := strings.TrimPrefix(strings.Trim(whole, ""), "Bearer ")
					var claims auth.Claims
					var err error

					ctx, claims, err = jwtVerifier.Verify(ctx, rawIDToken)
					if err != nil {
						log.Logger(ctx).Error("Invalid JWT Bearer")
						return errors.Forbidden(common.SERVICE_API_NAMESPACE_, "Invalid JWT Bearer")
					}

					log.Logger(ctx).Debug("Received API request with JWT Payload", zap.String("username", claims.Name))

					jwtValid = true
				}
			}

			if !jwtValid && !allowAnon {
				log.Logger(ctx).Error("Could not get a valid JWT")
				return errors.Forbidden(common.SERVICE_API_NAMESPACE_, "Could not get a valid JWT")
			}

			return h(ctx, req, rsp)

		}
	}
}
