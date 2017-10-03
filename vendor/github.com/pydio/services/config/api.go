package config

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service"

	"go.uber.org/zap"

	"github.com/micro/go-micro"
	"github.com/micro/go-micro/errors"
	"github.com/micro/micro/api/proto"

	"github.com/micro/cli"
	"github.com/pydio/services/common"
	"golang.org/x/net/context"

	"github.com/micro/config-srv/proto/config"
	"github.com/micro/go-os/config/proto"
)

type Config struct {
	client go_micro_srv_config_config.ConfigClient
}

type data struct {
	Service string                        `json:"name"`
	Path    string                        `json:"path"`
	Change  *go_micro_os_config.ChangeSet `json:"change"`
}

func apiBuilder(service micro.Service) interface{} {
	return &Config{
		client: go_micro_srv_config_config.NewConfigClient(common.SERVICE_CONFIG, service.Client()),
	}
}

// Put configuration in the database
func (s *Config) Put(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	log.Logger(ctx).Info("Put")

	var d data
	err := json.Unmarshal([]byte(req.Body), &d)
	if err != nil {
		return err
	}

	service := d.Service
	path := d.Path
	change := d.Change

	if path == "" {
		path = "service"
	}

	_, err = s.client.Create(ctx, &go_micro_srv_config_config.CreateRequest{
		Change: &go_micro_srv_config_config.Change{
			Id:        "services/" + service,
			Path:      path,
			Author:    "api",
			Comment:   "Initial load via api",
			ChangeSet: change,
		},
	})
	if err != nil {
		detail := errors.Parse(err.Error())
		if strings.HasPrefix(detail.Detail, "Error 1062") {
			_, err = s.client.Update(ctx, &go_micro_srv_config_config.UpdateRequest{
				Change: &go_micro_srv_config_config.Change{
					Id:        "services/" + service,
					Path:      path,
					Author:    "api",
					Comment:   "Updating configuration data",
					ChangeSet: change,
				},
			})
		}
	}
	if err != nil {
		return err
	}

	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

func (s *Config) Search(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	var d data
	err := json.Unmarshal([]byte(req.Body), &d)
	if err != nil {
		return err
	}

	log.Logger(ctx).Info("Search", zap.Any("data", d))

	response, err := s.client.Search(ctx, &go_micro_srv_config_config.SearchRequest{
		Id: "services/" + d.Service,
	})

	if err != nil {
		log.Logger(ctx).Error("Search", zap.Error(err))
		return err
	}

	configs := response.Configs

	if len(configs) > 1 {
		return errors.InternalServerError(common.SERVICE_CONFIG, "Could not retrieve single configuration")
	}

	changeSet := response.Configs[0].GetChangeSet()

	var c map[string]interface{}
	r := strings.NewReader(changeSet.Data)
	dec := json.NewDecoder(r)
	dec.Decode(&c)

	rsp.StatusCode = 200

	buf := new(bytes.Buffer)

	enc := json.NewEncoder(buf)
	data := map[string]interface{}{"success": true, "configs": c}
	enc.Encode(data)

	rsp.Body = buf.String()

	return nil
}

// Delete configuration in the database
func (s *Config) Delete(ctx context.Context, req *go_micro_api.Request, rsp *go_micro_api.Response) error {

	log.Logger(ctx).Info("Delete")

	var d data
	err := json.Unmarshal([]byte(req.Body), &d)
	if err != nil {
		return err
	}

	_, err = s.client.Delete(ctx, &go_micro_srv_config_config.DeleteRequest{
		Change: &go_micro_srv_config_config.Change{
			Id:        "services/" + d.Service,
			Path:      "service",
			Author:    "api",
			Comment:   "Deletion from api",
			ChangeSet: d.Change,
		},
	})

	if err != nil {
		return err
	}

	rsp.StatusCode = 200
	rsp.Body = `{"Success":"True"}`

	return nil
}

// NewAPIService for the config service
func NewAPIService(ctx *cli.Context) (micro.Service, error) {

	srv := service.NewAPIService(apiBuilder, micro.Name(common.SERVICE_API_NAMESPACE_+"config"))

	return srv, nil

}
