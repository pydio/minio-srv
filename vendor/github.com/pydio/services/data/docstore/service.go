package docstore

import (
	"path"

	"github.com/micro/cli"
	"github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/proto/docstore"
	"github.com/pydio/services/common/service"
)

func NewDocumentStoreService(ctx *cli.Context) (micro.Service, error) {

	dataDir, err := config.ApplicationDataDir()
	if err != nil {
		return nil, err
	}
	store, err := NewBoltStore(path.Join(dataDir, "docstore.db"))
	if err != nil {
		return nil, err
	}
	indexer, err := NewBleveEngine(path.Join(dataDir, "docstore.bleve"))
	handler := &Handler{
		Db:      store,
		Indexer: indexer,
	}

	srv := service.NewService(
		micro.Name(common.SERVICE_DOCSTORE),
		micro.BeforeStop(handler.Close),
	)

	docstore.RegisterDocStoreHandler(srv.Server(), handler)

	return srv, nil
}
