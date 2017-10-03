package views

import (
	"errors"
	"github.com/pydio/services/common/log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/client/grpc"
	"github.com/pydio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/object"
	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"github.com/micro/go-micro/client"
//	"fmt"
)

type sourceAlias struct{
	dataSource string
	bucket 	   string
}

type ClientsPool struct {
	Clients         map[string]*minio.Core
	TreeClient      tree.NodeProviderClient
	TreeClientWrite tree.NodeReceiverClient
	dsBuckets       map[string]string
	dsEncrypted     map[string]bool
	aliases 		map[string]sourceAlias

	genericClient   client.Client
	configMutex *sync.Mutex
	watcher     registry.Watcher
}

func NewClientsPool(watchRegistry bool) (pool *ClientsPool) {

	log.Logger(context.Background()).Info("--- CREATING CLIENTS POOL")

	pool = &ClientsPool{
		Clients:     make(map[string]*minio.Core),
		dsBuckets:   make(map[string]string),
		dsEncrypted: make(map[string]bool),
		aliases:     make(map[string]sourceAlias),
	}


	grpcClient := grpc.NewClient()
	pool.genericClient = grpcClient
	pool.TreeClient = tree.NewNodeProviderClient(common.SERVICE_TREE, grpcClient)
	pool.TreeClientWrite = tree.NewNodeReceiverClient(common.SERVICE_TREE, grpcClient)
	pool.configMutex = &sync.Mutex{}

	pool.listDatasources()
	if watchRegistry {
		go pool.watchRegistry()
	}

	return pool
}

func (p *ClientsPool) Close() {
	if p.watcher != nil {
		p.watcher.Stop()
	}
}

func (p *ClientsPool) GetDataSourceInfo(dsName string) (DSInfo, error) {

	if cl, ok := p.Clients[dsName]; ok {
		//log.Logger(context.Background()).Debug("GetDataSourceInfo: Client for " + dsName + ":" + fmt.Sprintf("%p", cl))
		return DSInfo{
			Client:    cl,
			Bucket:    p.dsBuckets[dsName],
			Encrypted: p.dsEncrypted[dsName],
		}, nil
	} else if alias, aOk := p.aliases[dsName]; aOk {
		dsi, e := p.GetDataSourceInfo(alias.dataSource)
		if e != nil {
			return dsi, e
		}
		//log.Logger(context.Background()).Debug("GetDataSourceInfo: Client for ALIAS " + dsName + ":" + fmt.Sprintf("%p", dsi.Client))
		return DSInfo{
			Client: dsi.Client,
			Bucket: alias.bucket,
			Encrypted: false,
		}, nil

	} else {
		return DSInfo{}, errors.New("Cannot find DataSource " + dsName)
	}

}

func (p *ClientsPool) listDatasources() {

	microClient := grpc.NewClient()
	otherServices, _ := microClient.Options().Registry.ListServices()
	indexServices := filterServices(otherServices, func(v string) bool {
		return strings.Contains(v, common.SERVICE_SYNC_)
	})

	ctx := context.Background()
	for _, indexService := range indexServices {

		dataSourceName := strings.TrimLeft(indexService, common.SERVICE_SYNC_)
		s3endpointClient := object.NewS3EndpointClient(common.SERVICE_OBJECTS_+dataSourceName, microClient)
		response, err := s3endpointClient.GetHttpURL(ctx, &object.GetHttpUrlRequest{})
		if err == nil && response.URL != "" {
			p.createClientsForDataSource(dataSourceName, response.URL, response.Encrypt)
		}

	}
	p.registerAlternativeClient(common.PYDIO_THUMBSTORE_NAMESPACE, microClient)
	p.registerAlternativeClient(common.PYDIO_DOCSTORE_BINARIES_NAMESPACE, microClient)
	p.registerAlternativeClient(common.PYDIO_VERSIONS_NAMESPACE, microClient)
}

func (p *ClientsPool) registerAlternativeClient(namespace string, microClient client.Client) error {
	dataSource, bucket, err := GetGenericStoreClientConfig(context.Background(), namespace, microClient)
	if err != nil {
		return err
	}
	p.configMutex.Lock()
	defer p.configMutex.Unlock()
	p.aliases[namespace] = sourceAlias{
		dataSource:dataSource,
		bucket:bucket,
	}
	return nil
}


func (p *ClientsPool) watchRegistry() {

	microClient := grpc.NewClient()
	watcher, err := microClient.Options().Registry.Watch()
	p.watcher = watcher
	if err != nil {
		return
	}
	for {
		result, err := watcher.Next()
		if result != nil && err == nil {
			service := result.Service
			if strings.Contains(service.Name, common.SERVICE_SYNC_) {
				dsName := strings.TrimPrefix(service.Name, common.SERVICE_SYNC_)
				if result.Action == "update" {
					continue
				}
				log.Logger(context.TODO()).Debug("Registry action", zap.String("action", result.Action), zap.String("dsname", dsName))
				if _, ok := p.Clients[dsName]; ok && result.Action == "delete" {
					// Reset list
					p.configMutex.Lock()
					delete(p.Clients, dsName)
					delete(p.dsBuckets, dsName)
					p.configMutex.Unlock()
				}
				p.listDatasources()
			}
		}
	}

}

func (p *ClientsPool) createClientsForDataSource(dataSourceName string, s3URL string, encrypted bool, registerKey ...string) error {

	log.Logger(context.TODO()).Debug("Adding dataSource", zap.String("dsname", dataSourceName), zap.String("s3url", s3URL))
	url, bucket := filepath.Split(s3URL)

	ds1, err := minio.NewCore(url, dataSourceName, dataSourceName+"secret", false)
	if err != nil {
		return err
	}

	p.configMutex.Lock()
	reg := dataSourceName
	if len(registerKey) > 0 {
		reg = registerKey[0]
	}
	p.Clients[reg] = ds1
	p.dsBuckets[reg] = bucket
	p.dsEncrypted[reg] = encrypted
	p.configMutex.Unlock()

	return nil

}

func filterServices(vs []*registry.Service, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v.Name) {
			vsf = append(vsf, v.Name)
		}
	}
	return vsf
}
