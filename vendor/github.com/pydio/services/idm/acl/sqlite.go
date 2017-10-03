package acl

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var sqliteSchema = []string{
	`CREATE TABLE IF NOT EXISTS acl_nodes (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid         VARCHAR(500) NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS acl_workspaces (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace    VARCHAR(500) NOT NULL
	);`,

	`CREATE TABLE IF NOT EXISTS acls (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id      INTEGER,
		action_name  VARCHAR(500),
		action_value VARCHAR(500),
		role_id      INTEGER,
		workspace_id INTEGER,

		FOREIGN KEY (node_id) REFERENCES acl_nodes(id),
		FOREIGN KEY (workspace_id) REFERENCES acl_workspaces(id),

		UNIQUE(node_id, action_name, role_id, workspace_id)
	);`,
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
