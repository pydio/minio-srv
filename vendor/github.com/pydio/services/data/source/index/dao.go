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
	"github.com/pydio/services/common/sql"
	"github.com/pydio/services/data/source/index/utils"
)

// DAO interface
type DAO interface {
	sql.DAO

	// Simple Add / Set / Delete
	AddNode(*utils.TreeNode) error
	SetNode(*utils.TreeNode) error
	DelNode(*utils.TreeNode) error

	// Batch Add / Set / Delete
	GetNodes(...utils.MPath) chan *utils.TreeNode
	SetNodes(string, int) sql.BatchSender

	// Getters
	GetNode(utils.MPath) (*utils.TreeNode, error)
	GetNodeByUUID(string) (*utils.TreeNode, error)
	GetNodeChild(utils.MPath, string) (*utils.TreeNode, error)
	GetNodeLastChild(utils.MPath) (*utils.TreeNode, error)
	GetNodeFirstAvailableChildIndex(utils.MPath) (uint64, error)
	GetNodeChildren(utils.MPath) chan *utils.TreeNode
	GetNodeTree(utils.MPath) chan *utils.TreeNode
}
