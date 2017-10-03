package acl

import (
	"sync/atomic"

	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var (
	mu atomic.Value

	mysqlSchema = []string{
		`CREATE TABLE IF NOT EXISTS acl_nodes (
			id           BIGINT NOT NULL AUTO_INCREMENT,
			uuid         VARCHAR(500) NOT NULL,

			PRIMARY KEY (id),
			UNIQUE(uuid)
		);`,

		`CREATE TABLE IF NOT EXISTS acl_workspaces (
			id           BIGINT NOT NULL AUTO_INCREMENT,
			name         VARCHAR(500) NOT NULL,

			PRIMARY KEY (id),
			UNIQUE(name)
		);`,

		`CREATE TABLE IF NOT EXISTS acls (
		  	id           BIGINT NOT NULL AUTO_INCREMENT,
			action_name  VARCHAR(500),
			action_value VARCHAR(500),
			role_id      BIGINT NOT NULL DEFAULT 0,
			node_id      BIGINT NOT NULL DEFAULT 0,
			workspace_id BIGINT NOT NULL DEFAULT 0,

			PRIMARY KEY (id),

			FOREIGN KEY acl_f1 (node_id) REFERENCES acl_nodes(id),
			FOREIGN KEY acl_f2 (workspace_id) REFERENCES acl_workspaces(id),

			CONSTRAINT acls_u1 UNIQUE(node_id, action_name, role_id, workspace_id)
		);`,

		`INSERT INTO acl_workspaces (id, name) VALUES (-1, "") ON DUPLICATE KEY UPDATE name = ""`,
		`INSERT INTO acl_nodes (id, uuid) VALUES (-1, "") ON DUPLICATE KEY UPDATE uuid = ""`,
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
