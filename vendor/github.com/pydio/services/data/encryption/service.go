package encryption

import (
	"context"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/data/encryption/dao"
	h "github.com/pydio/services/data/encryption/handler"
)

func StartEncryptionKeyService(ctx context.Context) (micro.Service, error) {

	var (
		d                  dao.DAO
		nodeReceiverClient tree.NodeReceiverClient
		nodeProviderClient tree.NodeProviderClient
	)

	srv := service.NewService(
		micro.Name(common.SERVICE_ENCRYPTION),
	)

	nodeReceiverClient = tree.NewNodeReceiverClient(common.SERVICE_ENCRYPTION, srv.Client())
	nodeProviderClient = tree.NewNodeProviderClient(common.SERVICE_ENCRYPTION, srv.Client())

	d, err := dao.NewDAO("mem")
	if err != nil {
		return nil, err
	}

	handler := &h.KeyManagerHandler{
		Dao:                &d,
		NodeReceiverClient: nodeReceiverClient,
		NodeProviderClient: nodeProviderClient,
	}

	tree.RegisterFileKeyManagerHandler(srv.Server(), handler)

	return srv, nil

}
