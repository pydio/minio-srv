package cmd

import (
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/client/grpc"
	"github.com/minio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/object"
	"golang.org/x/net/context"
	"path/filepath"
	"strings"
	"github.com/pydio/services/workers/jobs/images"
)

func (l *pydioObjects) listDatasources() {

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
			l.createClientsForDataSource(dataSourceName, response.URL)
		}

	}
	l.createThumbStoreClient()
}

func (l *pydioObjects) createThumbStoreClient() {

	url, bucket, apiKey, apiSecret, err := images.GetThumbStoreClientConfig(context.Background())
	if err == nil {
		l.configMutex.Lock()
		defer l.configMutex.Unlock()
		l.dsBuckets[common.PYDIO_THUMBSTORE_NAMESPACE] = bucket

		client, e := minio.NewCore(url, apiKey, apiSecret, false)
		if e == nil {
			l.Clients[common.PYDIO_THUMBSTORE_NAMESPACE] = client
		}
	}

}

func (l *pydioObjects) watchRegistry() {

	microClient := grpc.NewClient()
	watcher, err := microClient.Options().Registry.Watch()
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
				log.Printf("\n%v %v", result.Action, dsName)
				if _, ok := l.Clients[dsName]; ok && result.Action == "delete" {
					// Reset list
					l.configMutex.Lock()
					delete(l.Clients, dsName)
					delete(l.anonClients, dsName)
					delete(l.dsBuckets, dsName)
					l.configMutex.Unlock()
				}
				l.listDatasources()
			}
		}
	}

}

func (l *pydioObjects) createClientsForDataSource(dataSourceName string, s3URL string, registerKey ...string) error {

	log.Printf("\n-- Adding dataSource %v with URL %v", dataSourceName, s3URL)
	url, bucket := filepath.Split(s3URL)

	ds1, err := minio.NewCore(url, dataSourceName, dataSourceName+"secret", false)
	if err != nil {
		return err
	}
	anonClient, err := minio.NewCore(url, "", "", false)
	if err != nil {
		return err
	}

	l.configMutex.Lock()
	reg := dataSourceName
	if len(registerKey) > 0 {
		reg = registerKey[0]
	}
	l.anonClients[reg] = anonClient
	l.Clients[reg] = ds1
	l.dsBuckets[reg] = bucket
	l.configMutex.Unlock()

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
