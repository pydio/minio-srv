package cmd

import (
	"encoding/json"
	"github.com/micro/config-srv/proto/config"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/client/grpc"
	"github.com/minio/minio-go"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/object"
	"golang.org/x/net/context"
	"path/filepath"
	"strings"
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

	microClient := grpc.NewClient()
	configClient := go_micro_srv_config_config.NewConfigClient(common.SERVICE_CONFIG, microClient)
	resp, err := configClient.Read(context.Background(), &go_micro_srv_config_config.ReadRequest{
		Id:   "services/pydio.thumbs_store",
		Path: "service",
	})
	if err == nil && resp.GetChange() != nil {
		stringData := resp.GetChange().GetChangeSet().Data
		log.Println(stringData)
		config := make(map[string]interface{})
		json.Unmarshal([]byte(stringData), &config)
		ds := config["datasource"].(string)
		bucket := config["bucket"].(string)
		var ok bool
		if _, ok = l.Clients[ds]; !ok {
			s3endpointClient := object.NewS3EndpointClient(common.SERVICE_OBJECTS_+ds, microClient)
			response, err := s3endpointClient.GetHttpURL(context.Background(), &object.GetHttpUrlRequest{})
			if err == nil && response.URL != "" {
				l.createClientsForDataSource(ds, response.URL, common.PYDIO_THUMBSTORE_NAMESPACE)
			}
		} else {
			l.Clients[common.PYDIO_THUMBSTORE_NAMESPACE] = l.Clients[ds]
			l.anonClients[common.PYDIO_THUMBSTORE_NAMESPACE] = l.anonClients[ds]
		}
		l.dsBuckets[common.PYDIO_THUMBSTORE_NAMESPACE] = bucket
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
