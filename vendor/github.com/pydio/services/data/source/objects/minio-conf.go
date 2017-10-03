package objects

import (
	"encoding/json"
	"github.com/pydio/minio-priv/cmd"
	"github.com/pydio/services/common/config"
	"os"
	"path"
)

func CreateMinioConfigFile(serviceId string, accessKey string, secretKey string) (configDir string, err error) {

	configDir, e := config.ApplicationDataDir()
	if e != nil {
		return "", e
	}

	gatewayDir := path.Join(configDir, serviceId)
	gatewayFile := path.Join(gatewayDir, "config.json")

	if _, err := os.Stat(gatewayFile); os.IsNotExist(err) {
		e := os.MkdirAll(gatewayDir, 0755)
		if e != nil {
			return "", e
		}
		configuration := cmd.CreateEmptyMinioConfig()
		configuration.Credential.AccessKey = accessKey
		configuration.Credential.SecretKey = secretKey
		// Create basic config file
		data, _ := json.Marshal(configuration)
		if file, e := os.OpenFile(gatewayFile, os.O_CREATE|os.O_WRONLY, 0755); e == nil {
			defer file.Close()
			file.Write(data)
		} else {
			return "", e
		}
	}

	return gatewayDir, nil

}
