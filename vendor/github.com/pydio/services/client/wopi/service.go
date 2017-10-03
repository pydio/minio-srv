package wopi

import (
	"context"
	"fmt"
	"net/http"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/views"
)

var (
	treeClient  tree.NodeProviderClient
	viewsRouter *views.Router
)

func NewWOPIService(ctx context.Context, port int) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_API_NAMESPACE_ + "wopi"),
	)

	ctx = srv.Options().Context

	viewsRouter = views.NewUuidRouter(false, true)
	treeClient = tree.NewNodeProviderClient(common.SERVICE_TREE, srv.Client())

	router := NewRouter()
	defaultPort := 5014
	// if ctx.Int("wopi_port") > 0 {
	// 	defaultPort = ctx.Int("wopi_port")
	// }
	//
	// var config config.Map // TODO
	// if configPort := config.Get("port"); configPort != nil {
	// 	defaultPort = configPort.(int)
	// }

	go func() {
		log.Logger(ctx).Info(fmt.Sprintf("Starting Wopi Server on port %d", defaultPort))
		err := http.ListenAndServe(fmt.Sprintf(":%d", defaultPort), router)
		if err != nil {
			log.Logger(ctx).Error(err.Error())
		}
	}()

	return srv, nil
}
