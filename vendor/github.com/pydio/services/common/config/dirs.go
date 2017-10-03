package config

import (
	"github.com/shibukawa/configdir"
	"os"
)

func ApplicationDataDir() (string, error) {

	configDirs := configdir.New("Pydio", "Server")
	folders := configDirs.QueryFolders(configdir.Global)
	if len(folders) == 0 {
		folders = configDirs.QueryFolders(configdir.Local)
	}
	f := folders[0].Path
	err := os.MkdirAll(f, 0777)
	return f, err

}