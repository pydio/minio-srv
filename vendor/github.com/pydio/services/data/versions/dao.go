package versions

import "github.com/pydio/services/common/proto/tree"

type DAO interface{
	GetLastVersion(nodeUuid string) (*tree.ChangeLog, error)
	GetVersions(nodeUuid string) (chan *tree.ChangeLog, chan bool)
	GetVersion(nodeUuid string, versionId string) (*tree.ChangeLog, error)
	StoreVersion(nodeUuid string, log *tree.ChangeLog) error
	DeleteVersionsForNode(nodeUuid string) error
}