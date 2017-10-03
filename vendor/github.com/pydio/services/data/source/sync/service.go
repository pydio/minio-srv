package sync

import (
	"path/filepath"

	"go.uber.org/zap"

	"errors"
	"strings"
	"time"

	micro "github.com/micro/go-micro"
	synccommon "github.com/pydio/poc/sync/common"
	"github.com/pydio/poc/sync/endpoints"
	sync "github.com/pydio/poc/sync/task"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	protosync "github.com/pydio/services/common/proto/sync"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/data/source/objects"
	"golang.org/x/net/context"
)

// NewSyncService definition
func NewSyncService(ctx context.Context, datasource string, normalizeS3 bool, fsWatch string) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_SYNC_+datasource),
		micro.Context(ctx),
	)

	ctx = srv.Options().Context

	url := objects.GetS3UrlWithRetries(ctx, common.SERVICE_OBJECTS_+datasource, srv.Client(), 0)
	if url == "" {
		return nil, errors.New("Could not contact associated Object service!")
	}

	host, bucketName := filepath.Split(url)

	var source synccommon.PathSyncTarget
	if fsWatch != "" {
		s3client, errs3 := endpoints.NewS3ClientFSWatch(host, datasource, datasource+"secret", bucketName, "", fsWatch)
		if errs3 != nil {
			return nil, errs3
		}
		if normalizeS3 {
			s3client.ServerRequiresNormalization = true
		}
		source = s3client
	} else {
		s3client, errs3 := endpoints.NewS3Client(host, datasource, datasource+"secret", bucketName, "")
		if errs3 != nil {
			return nil, errs3
		}
		if normalizeS3 {
			s3client.ServerRequiresNormalization = true
		}
		source = s3client
	}

	indexClientWrite := tree.NewNodeReceiverClient(common.SERVICE_INDEX_+datasource, srv.Client())
	indexClientRead := tree.NewNodeProviderClient(common.SERVICE_INDEX_+datasource, srv.Client())

	if test := contactIndexWithRetries(indexClientRead, 0); !test {
		return nil, errors.New("Could not connect to Index service!")
	}

	target := NewIndexEndpoint(datasource, indexClientRead, indexClientWrite)

	syncTask := sync.NewSync(source, target)
	syncTask.Direction = "left"

	syncHandler := &Handler{
		S3client:    source,
		IndexClient: indexClientRead,
		SyncTask:    syncTask,
	}
	tree.RegisterNodeProviderHandler(srv.Server(), syncHandler)
	tree.RegisterNodeReceiverHandler(srv.Server(), syncHandler)
	protosync.RegisterSyncEndpointHandler(srv.Server(), syncHandler)

	syncTask.Start()
	_, e := syncTask.InitialSnapshots(false)
	if e != nil {
		log.Logger(ctx).Error("Cannot run initial snapshots, maybe the source or target is dead ?", zap.Error(e))
	}

	return srv, nil
}

func contactIndexWithRetries(client tree.NodeProviderClient, count int) bool {

	if count > 4 {
		return false
	}

	_, e := client.ReadNode(context.Background(), &tree.ReadNodeRequest{Node: &tree.Node{Path: ""}})
	// TODO : find a better way to discriminate between a real connection error and a node not found error
	if e == nil || strings.Contains(e.Error(), "Could not retrieve node") {
		return true
	}

	log.Logger(context.TODO()).Error("Could not contact Index service, retrying in 3s...")

	time.Sleep(3 * time.Second)
	return contactIndexWithRetries(client, count+1)
}
