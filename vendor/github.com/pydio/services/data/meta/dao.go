package meta

import "github.com/pydio/services/common/sql"

type DAO interface {
	sql.DAO

	SetMetadata(nodeId string, metadata map[string]string) (err error)
	GetMetadata(nodeId string) (metadata map[string]string, err error)
	ListMetadata(query string) (metadataByUuid map[string]map[string]string, err error)
}
