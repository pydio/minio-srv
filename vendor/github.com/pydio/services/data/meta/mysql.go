package meta

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var (
	// TreeSchema definition
	mysqlSchema = []string{
		`CREATE TABLE IF NOT EXISTS meta (
			node_id varchar(255) not null,
			namespace varchar(255) not null,
			author varchar(255),
			timestamp int(11),
			data blob,
			format varchar(255),
			unique(node_id, namespace),
			index(timestamp),
			index(author)
		);`,
	}
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

	if err := database.CreateSchema(mysqlSchema...); err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Error while creating schema", err)
	}

	dao.sqlimpl.Init(database, options)

	return nil
}
