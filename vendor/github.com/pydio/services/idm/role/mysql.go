package role

import (
	"sync/atomic"

	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var (
	mu atomic.Value

	mysqlSchema = `
		CREATE TABLE IF NOT EXISTS roles (
			id   INTEGER PRIMARY KEY AUTO_INCREMENT NOT NULL,
			name VARCHAR(500) NOT NULL,

			UNIQUE (name)
		);`
)

// NewMySQL implementation of the DAO
func NewMySQL() DAO {
	return new(mysqlimpl)
}

// mysqlimpl of the Mysql interface
type mysqlimpl struct {
	sqlimpl
}

// Init of the MySQL DAO
func (dao *mysqlimpl) Init(database sql.Provider, options common.Manager) error {

	if err := database.CreateSchema(mysqlSchema); err != nil {
		return errors.InternalServerError(common.SERVICE_ROLE, "Error while creating schema", err)
	}

	dao.sqlimpl.Init(database, options)

	return nil
}
