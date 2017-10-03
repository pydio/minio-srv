package meta

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
)

type stubImpl struct {
	DAO
	internalMeta map[string]map[string]string
}

func (dao *stubImpl) SetMetadata(nodeId string, metadata map[string]string) (err error) {
	if dao.internalMeta == nil {
		dao.internalMeta = make(map[string]map[string]string)
	}
	if _, exist := dao.internalMeta[nodeId]; exist && len(metadata) == 0 {
		dao.internalMeta[nodeId] = nil
		return nil
	}
	dao.internalMeta[nodeId] = metadata
	return nil
}

func (dao *stubImpl) GetMetadata(nodeId string) (metadata map[string]string, err error) {

	if nodeId == "stubbed-unique-id" {
		metadata = make(map[string]string)
		metadata["namespace"] = `{"key":"value"}`
		return metadata, nil
	} else if dao.internalMeta == nil {
		err = errors.NotFound(common.SERVICE_META, "Metadata not initialized for stub ")
		return nil, err
	} else if m, exist := dao.internalMeta[nodeId]; exist {
		return m, nil
	} else {
		err = errors.NotFound(common.SERVICE_META, "Cannot find metadata for node "+nodeId)
		return nil, err
	}

}

func (dao *stubImpl) ListMetadata(query string) (map[string]map[string]string, error) {
	return nil, errors.NotFound(common.SERVICE_META, "Stub::ListMetadata is not implemented")
}
