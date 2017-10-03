package user

import (
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/sql"
)

// DAO interface
type DAO interface {
	sql.DAO

	Add(interface{}) error
	Del(sql.Enquirer) (numRows int64, e error)
	Search(sql.Enquirer, *[]interface{}) error
	Bind(userName string, password string) (*idm.User, error)
}
