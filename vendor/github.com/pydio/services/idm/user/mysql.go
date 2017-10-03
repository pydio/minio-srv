package user

import (
	"sync/atomic"

	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var (
	mu atomic.Value

	mysqlSchema = []string{`
		CREATE TABLE IF NOT EXISTS users (
			id         INTEGER PRIMARY KEY AUTO_INCREMENT NOT NULL,
			login      VARCHAR(255) NOT NULL,
			password   VARCHAR(255) NOT NULL,
			group_path VARCHAR(255),

			UNIQUE (login)
		);`,

		`CREATE TABLE IF NOT EXISTS users_attributes (
			id         INTEGER  NOT NULL,
			name       VARCHAR(255) NOT NULL,
			value      VARCHAR(255),

			PRIMARY KEY (id, name),
			FOREIGN KEY (id) REFERENCES users(id)
		)`,
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
