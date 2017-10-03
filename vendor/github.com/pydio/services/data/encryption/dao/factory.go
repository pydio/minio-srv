package dao

import (
	"github.com/pydio/services/data/encryption/dao/stub"
	"fmt"
)

func NewDAO(e string) (DAO, error) {
	switch e {
	case "mem":
		return stub.NewMemUKM()

	default:
		return nil, fmt.Errorf("Engine %s not supported", e)
	}
}