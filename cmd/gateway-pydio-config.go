package cmd

import (
	"path/filepath"
	"github.com/minio/minio-go"
	"strings"
	"github.com/micro/go-plugins/client/grpc"
	"github.com/micro/go-micro/registry"
	"github.com/pydio/services/common/proto/object"
	"github.com/pydio/services/common"
	"golang.org/x/net/context"
)

func (l *pydioObjects) listDatasources(){

	microClient := grpc.NewClient()
	otherServices, _ := microClient.Options().Registry.ListServices()
	indexServices := filterServices(otherServices, func(v string) bool {
		return strings.Contains(v, common.SERVICE_SYNC_)
	})

	ctx := context.Background()
	for _, indexService := range indexServices {

		dataSourceName := strings.TrimLeft(indexService, common.SERVICE_SYNC_)
		s3endpointClient := object.NewS3EndpointClient(common.SERVICE_OBJECTS_ + dataSourceName, microClient)
		response, err := s3endpointClient.GetHttpURL(ctx, &object.GetHttpUrlRequest{})
		if err == nil && response.URL != ""{
			l.createClientsForDataSource(dataSourceName, response.URL)
		}

	}
}

func (l *pydioObjects) watchRegistry(){

	microClient := grpc.NewClient()
	watcher, err := microClient.Options().Registry.Watch()
	if err != nil{
		return
	}
	for {
		result, err := watcher.Next()
		if result != nil && err == nil{
			service := result.Service
			if strings.Contains(service.Name, common.SERVICE_SYNC_){
				dsName := strings.TrimPrefix(service.Name, common.SERVICE_SYNC_)
				if result.Action == "update"{
					continue
				}
				log.Printf("\n%v %v", result.Action, dsName)
				if _, ok := l.Clients[dsName]; ok && result.Action == "delete"{
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

func (l *pydioObjects) createClientsForDataSource(dataSourceName string, s3URL string) error {

	log.Printf("\n-- Adding dataSource %v with URL %v", dataSourceName, s3URL)
	url, bucket := filepath.Split(s3URL)

	ds1, err := minio.NewCore(url, dataSourceName, dataSourceName + "secret", false)
	if err != nil{
		return err
	}
	anonClient, err := minio.NewCore(url, "", "", false)
	if err != nil {
		return err
	}

	l.configMutex.Lock()
	l.anonClients[dataSourceName] = anonClient
	l.Clients[dataSourceName] = ds1
	l.dsBuckets[dataSourceName] = bucket
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
