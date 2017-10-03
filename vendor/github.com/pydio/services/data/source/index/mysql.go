/*
 * Copyright 2007-2017 Abstrium <contact (at) pydio.com>
 * This file is part of Pydio.
 *
 * Pydio is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com/>.
 */
package index

import (
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var (
	// TreeSchema definition
	mysqlSchema = []string{
		`CREATE TABLE IF NOT EXISTS tree (
			uuid  VARCHAR(128)      NOT NULL,
            level SMALLINT          NOT NULL,

			hash  BIGINT            NOT NULL,
			mpath TEXT              NOT NULL,
			rat   BLOB              NOT NULL,

            CONSTRAINT tree_pk PRIMARY KEY (uuid),
			CONSTRAINT tree_u1 UNIQUE (hash)
        );`,

		`CREATE TABLE IF NOT EXISTS nodes (
            uuid     VARCHAR(128) NOT NULL,
            name     VARCHAR(255) NOT NULL,
            leaf     TINYINT(1),
            mtime    INT NOT NULL,
            etag     VARCHAR(255),
            size     BIGINT,
            mode     VARCHAR(10),

			CONSTRAINT node_pk PRIMARY KEY (uuid)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`,

		`CREATE TABLE IF NOT EXISTS commits (
			uuid     VARCHAR(128) NOT NULL,
			cid      VARCHAR(128) NOT NULL,
			version  TINYINT(1),
			mtime    INT NOT NULL,
			description VARCHAR(255),
			data     BLOB NULL,
			CONSTRAINT commits_u1 UNIQUE (uuid,cid,version)
		) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;`,

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
