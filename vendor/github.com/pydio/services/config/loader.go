package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/micro/config-srv/handler"
	config "github.com/micro/config-srv/proto/config"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"

	"github.com/micro/go-os/config/proto"
)

var services map[string]map[string]interface{}

// Config is the server definition
type Loader struct {
	Services map[string]map[string]interface{} `json:"services" yaml:"services"`
}

// GetConfigServiceDSN returns the datasourcename for the service Config
func (l *Loader) GetConfigServiceDSN() (string, error) {

	if s, ok := l.Services[common.SERVICE_CONFIG]; ok {
		if dsn, ok := s["dsn"].(string); ok {
			return dsn, nil
		}
	}
	return "", errors.NotFound(common.SERVICE_CONFIG, "DSN Key for Config Service was not found. Please add at least one entry for pydio.service.configs in your configuration file")
}

func (l *Loader) filler(h *handler.Config) func() error {
	return func() error {
		for k, v := range l.Services {
			if k != common.SERVICE_CONFIG {
				var b []byte
				buf := bytes.NewBuffer(b)
				enc := json.NewEncoder(buf)

				if err := enc.Encode(v); err == nil {
					h.Create(context.Background(), &config.CreateRequest{
						Change: &config.Change{
							Id:      "services/" + k,
							Path:    "service",
							Author:  "cli",
							Comment: "Initial load",
							ChangeSet: &go_micro_os_config.ChangeSet{
								Data: string(buf.Bytes()),
							},
						},
					}, &config.CreateResponse{})
				} else {
					return errors.InternalServerError(common.SERVICE_CONFIG, "Could not encode data to json")
				}
			}
		}

		return nil
	}
}

func (l *Loader) parser(filename string) func() error {
	return func() error {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}

		if strings.HasSuffix(filename, ".json") {
			if err := json.Unmarshal(data, &l); err != nil {
				return err
			}
		} else if strings.HasSuffix(filename, ".yaml") {
			if err := yaml.Unmarshal(data, &l); err != nil {
				return err
			}
		} else {
			return errors.NotFound(common.SERVICE_CONFIG, "Cannot find configuration file "+filename)
		}

		return nil
	}
}
