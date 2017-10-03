package user

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var sqliteSchema = []string{`
	CREATE TABLE IF NOT EXISTS users (
		id         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
		login      VARCHAR(255) NOT NULL,
		password   VARCHAR(255) NOT NULL,
		group_path VARCHAR(255)
	);`,

	`CREATE TABLE IF NOT EXISTS users_attributes (
		id         INTEGER NOT NULL,
		name       VARCHAR(255) NOT NULL,
		value      VARCHAR(255),

		PRIMARY KEY (id, name),
		FOREIGN KEY (id) REFERENCES users(id)
	)`,
}

// NewSQLite implementation of the DAO
func NewSQLite() DAO {
	return new(sqliteimpl)
}

// sqliteimpl of the Mysql interface
type sqliteimpl struct {
	sqlimpl
}

// Init of the SQLite DAO
func (dao *sqliteimpl) Init(database sql.Provider, options common.Manager) error {

	if err := database.CreateSchema(sqliteSchema...); err != nil {
		return errors.InternalServerError(common.SERVICE_INDEX_, "Error while creating schema", err)
	}

	dao.sqlimpl.Init(database, options)

	dao.SQLConn.DB = database.GetConn()
	dao.SQLConn.Stmts = database.GetStmts()

	return nil
}
