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
	databasesql "database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"

	"time"

	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/common/sql"
	"github.com/pydio/services/data/source/index/utils"
)

var mu atomic.Value

const batchLen = 20

var (
	dbName    = "micro"
	tableName = "tree"

	queries = map[string]string{}

	batch = "?" + strings.Repeat(", ?", batchLen-1)
)

// BatchSend sql structure
type BatchSend struct {
	in  chan *utils.TreeNode
	out chan error
}

func init() {
	queries["insertTree"] = `
		insert into tree (uuid, level, hash, mpath, rat)
		values (?, ?, ?, ?, ?)`

	queries["insertNode"] = `
		insert into nodes (uuid, name, leaf, mtime, etag, size, mode)
		values (?, ?, ?, ?, ?, ?, ?)`

	queries["insertCommit"] = `
		insert into commits (uuid, cid, version, mtime, description)
		values (?, ?, ?, ?, ?)`

	queries["insertCommitWithData"] = `
		insert into commits (uuid, cid, version, mtime, description, data)
		values (?, ?, ?, ?, ?, ?)`

	queries["updateTree"] = `
		update tree set level = ?, hash = ?, mpath = ?, rat = ?
		where uuid = ?`

	queries["updateNode"] = `
		update nodes set name = ?, leaf = ?, mtime = ?, etag = ?, size = ?, mode = ?
		where uuid = ?`

	queries["updateNodes"] = `
		update nodes set mtime = ?, etag = ?, size = size + ?
		where uuid in (
			select uuid from tree where mpath in (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		)`

	queries["deleteTree"] = `
		delete from tree
		where mpath LIKE ?`

	queries["deleteNode"] = `
		delete from nodes
		where uuid in (
			select uuid
			from tree
			where mpath LIKE ?
		)`

	queries["deleteCommits"] = `
		delete from commits where uuid in (
			select uuid
			from tree
			where mpath LIKE ?
		)`

	queries["selectCommits"] = `
		select cid,mtime,description,data from commits where uuid = ? and version = 0
	`

	queries["selectNode"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
		from tree t, nodes n
		where t.mpath = ?
		and n.uuid = t.uuid`

	queries["selectNodeUuid"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
        from tree t, nodes n
		where t.uuid = ?
		and n.uuid = t.uuid`

	queries["selectNodes"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
		from tree t, nodes n
		where t.mpath in (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		and n.uuid = t.uuid
		order by t.mpath`

	queries["tree"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
		from tree t, nodes n
		where t.mpath LIKE ?
		and t.uuid = n.uuid
		and t.level >= ?
		order by t.mpath`

	queries["children"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
		from tree t, nodes n
		where t.mpath LIKE ?
		and t.uuid = n.uuid
		and t.level = ?
		order by n.name`

	queries["child"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
		from tree t, nodes n
		where t.mpath LIKE ?
		and t.uuid = n.uuid
		and t.level = ?
		and n.name like ?`

	queries["lastChild"] = `
		select t.uuid, t.level, t.mpath, t.rat, n.name, n.leaf, n.mtime, n.etag, n.size, n.mode
		from tree t, nodes n
		where t.mpath LIKE ?
		and t.uuid = n.uuid
		and t.level = ?
		order by t.mpath desc limit 1`

	queries["integrity1"] = `select count(uuid) from tree where uuid not in (select uuid from nodes)`
	queries["integrity2"] = `select count(uuid) from nodes where uuid not in (select uuid from tree)`
}

// Impl of the Mysql interface
type sqlimpl struct {
	sql.SQLConn
}

// Add to the mysql DB
func (dao *sqlimpl) Init(database sql.Provider, options common.Manager) error {

	if exclusive, ok := options.Get("exclusive").(bool); ok && exclusive {
		mu.Store(&sync.Mutex{})
	}

	if prepare, ok := options.Get("prepare").(bool); !ok || prepare {
		for key, query := range queries {
			if err := database.Prepare(key, query); err != nil {
				return errors.New("Error while preparing statements")
			}
		}
	}

	dao.SQLConn.DB = database.GetConn()
	dao.SQLConn.Stmts = database.GetStmts()

	return nil
}

// AddNode to the mysql database
func (dao *sqlimpl) AddNode(node *utils.TreeNode) error {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	var err error

	// Starting a transaction
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	// Checking transaction went fine
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	mTime := node.GetMTime()
	if mTime == 0 {
		mTime = time.Now().Unix()
	}

	if _, err = dao.GetStmts().Get("insertNode").Exec(
		node.Uuid,
		node.Name(),
		node.IsLeafInt(),
		mTime,
		node.GetEtag(),
		node.GetSize(),
		node.GetMode(),
	); err != nil {
		return err
	}

	if _, err = dao.GetStmts().Get("insertTree").Exec(
		node.Uuid,
		node.Level,
		node.MPath.Hash(),
		node.MPath.String(),
		node.Bytes(),
	); err != nil {
		return err
	}

	if err := dao.checkIntegrity("AddNode"); err != nil {
		return err
	}

	return nil
}

// SetNode in replacement of previous node
func (dao *sqlimpl) SetNode(node *utils.TreeNode) error {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	var err error

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if _, err = dao.GetStmts().Get("updateTree").Exec(
		node.Level,
		node.MPath.Hash(),
		node.MPath.String(),
		node.Bytes(),
		node.Uuid,
	); err != nil {
		return err
	}

	if _, err = dao.GetStmts().Get("updateNode").Exec(
		node.Name(),
		node.IsLeafInt(),
		node.MTime,
		node.Etag,
		node.Size,
		node.Mode,
		node.Uuid,
	); err != nil {
		return err
	}

	if checkErr := dao.checkIntegrity("SetNode End"); checkErr != nil {
		return checkErr
	}

	return nil
}

// SetNodes returns a channel and waits for arriving nodes before updating them in batch
func (dao *sqlimpl) SetNodes(etag string, deltaSize int) sql.BatchSender {

	db := dao.DB

	b := NewBatchSend()

	go func() {
		dao.lock()
		defer dao.unlock()

		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			b.out <- err
		}

		defer func() {
			if err != nil {
				tx.Rollback()
			} else {
				tx.Commit()
			}

			close(b.out)
		}()

		insert := func(args ...interface{}) {
			args = append([]interface{}{time.Now().Unix(), etag, deltaSize}, args...)
			if _, err = dao.GetStmts().Get("updateNodes").Exec(args...); err != nil {
				b.out <- err
			}
		}

		all := make([]interface{}, 0, batchLen)

		for node := range b.in {
			all = append(all, node.MPath.String())
			if len(all) == cap(all) {
				insert(all...)
				all = all[:0]
			}
		}

		if len(all) > 0 {
			for len(all) < cap(all) {
				all = append(all, "-1")
			}
			insert(all...)
		}
	}()

	return b
}

// DelNode from database
func (dao *sqlimpl) DelNode(node *utils.TreeNode) error {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	var err error

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	mpath := fmt.Sprintf("%s%%", node.MPath.String())

	if _, err = dao.GetStmts().Get("deleteNode").Exec(
		mpath,
	); err != nil {
		return err
	}

	if _, err = dao.GetStmts().Get("deleteTree").Exec(
		mpath,
	); err != nil {
		return err
	}

	if _, err = dao.GetStmts().Get("deleteCommits").Exec(
		mpath,
	); err != nil {
		return err
	}

	if errCheck := dao.checkIntegrity("DelNodeEnd " + node.Path); errCheck != nil {
		return errCheck
	}

	return nil
}

// GetNode from path
func (dao *sqlimpl) GetNode(path utils.MPath) (*utils.TreeNode, error) {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	node := utils.NewTreeNode()
	node.SetMPath(path...)

	row := db.QueryRow(queries["selectNode"], node.MPath.String())
	treeNode, err := dao.scanDbRowToTreeNode(row)
	if err != nil {
		return nil, err
	}
	return treeNode, nil
}

// GetNodeByUUID returns the node stored with the unique uuid
func (dao *sqlimpl) GetNodeByUUID(uuid string) (*utils.TreeNode, error) {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	row := db.QueryRow(queries["selectNodeUuid"], uuid)
	treeNode, err := dao.scanDbRowToTreeNode(row)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return treeNode, nil
}

// GetNodes List
func (dao *sqlimpl) GetNodes(mpathes ...utils.MPath) chan *utils.TreeNode {

	dao.lock()

	c := make(chan *utils.TreeNode)

	go func() {

		defer func() {
			close(c)
			dao.unlock()
		}()

		get := func(args ...interface{}) {
			rows, err := dao.GetStmts().Get("selectNodes").Query(args...)
			if err != nil {
				return
			}

			defer rows.Close()

			for rows.Next() {
				node, err := dao.scanDbRowToTreeNode(rows)
				if err != nil {
					break
				}

				c <- node
			}
		}

		all := make([]interface{}, 0, batchLen)

		for _, mpath := range mpathes {
			all = append(all, mpath.String())
			if len(all) == cap(all) {
				get(all...)
				all = all[:0]
			}
		}

		if len(all) > 0 {
			for len(all) < cap(all) {
				all = append(all, "-1")
			}
			get(all...)
		}
	}()

	return c
}

// GetNodeChild from node path whose name matches
func (dao *sqlimpl) GetNodeChild(reqPath utils.MPath, reqName string) (*utils.TreeNode, error) {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	node := utils.NewTreeNode()
	node.SetMPath(reqPath...)

	mpath := fmt.Sprintf("%s%%", node.MPath.String())

	row := db.QueryRow(queries["child"], mpath, len(reqPath)+1, reqName)
	treeNode, err := dao.scanDbRowToTreeNode(row)
	if err != nil {
		return nil, err
	}
	return treeNode, nil

}

// GetNodeLastChild from path
func (dao *sqlimpl) GetNodeLastChild(reqPath utils.MPath) (*utils.TreeNode, error) {

	dao.lock()
	defer dao.unlock()

	db := dao.DB

	node := utils.NewTreeNode()
	node.SetMPath(reqPath...)

	mpath := fmt.Sprintf("%s%%", node.MPath.String())

	row := db.QueryRow(queries["lastChild"], mpath, len(reqPath)+1)
	treeNode, err := dao.scanDbRowToTreeNode(row)
	if err != nil {
		return nil, err
	}

	return treeNode, nil
}

// GetNodeFirstAvailableChildIndex from path
func (dao *sqlimpl) GetNodeFirstAvailableChildIndex(reqPath utils.MPath) (uint64, error) {

	all := []int{}

	for node := range dao.GetNodeChildren(reqPath) {
		all = append(all, int(node.MPath.Index()))
	}

	if len(all) == 0 {
		return 1, nil
	}

	sort.Ints(all)
	max := all[len(all)-1]

	for i := 1; i <= max; i++ {
		found := false
		for _, v := range all {
			if i == v {
				// We found the entry, so next one
				found = true
				break
			}
		}

		if !found {
			// This number is not present, returning it
			return uint64(i), nil
		}
	}

	return uint64(max + 1), nil
}

// GetNodeChildren List
func (dao *sqlimpl) GetNodeChildren(path utils.MPath) chan *utils.TreeNode {

	dao.lock()

	db := dao.DB

	c := make(chan *utils.TreeNode)

	go func() {
		var rows *databasesql.Rows
		var err error

		defer func() {
			if rows != nil {
				rows.Close()
			}
			close(c)
			dao.unlock()
		}()

		node := utils.NewTreeNode()
		node.SetMPath(path...)

		mpath := fmt.Sprintf("%s%%", node.MPath.String())

		// First we check if we already have an object with the same key
		rows, err = db.Query(queries["children"], mpath, len(path)+1)
		if err != nil {
			return
		}

		for rows.Next() {
			treeNode, err := dao.scanDbRowToTreeNode(rows)
			if err != nil {
				break
			}

			c <- treeNode
		}
	}()

	return c
}

// GetNodeTree List from the path
func (dao *sqlimpl) GetNodeTree(path utils.MPath) chan *utils.TreeNode {

	dao.lock()

	db := dao.DB

	c := make(chan *utils.TreeNode)

	go func() {
		var rows *databasesql.Rows
		var err error

		defer func() {
			if rows != nil {
				rows.Close()
			}

			close(c)
			dao.unlock()
		}()

		node := utils.NewTreeNode()
		node.SetMPath(path...)

		mpath := fmt.Sprintf("%s%%", node.MPath.String())

		// First we check if we already have an object with the same key
		rows, err = db.Query(queries["tree"], mpath, len(path)+1)
		if err != nil {
			return
		}

		for rows.Next() {
			treeNode, err := dao.scanDbRowToTreeNode(rows)
			if err != nil {
				break
			}

			c <- treeNode
		}
	}()

	return c
}

func (dao *sqlimpl) scanDbRowToTreeNode(row sql.Scanner) (*utils.TreeNode, error) {
	var (
		uuid  string
		mpath string
		rat   []byte
		level uint32
		name  string
		leaf  int32
		mtime int64
		etag  string
		size  int64
		mode  int32
	)

	if err := row.Scan(&uuid, &level, &mpath, &rat, &name, &leaf, &mtime, &etag, &size, &mode); err != nil {
		return nil, err
	}
	nodeType := tree.NodeType_LEAF
	if leaf == 0 {
		nodeType = tree.NodeType_COLLECTION
	}

	node := utils.NewTreeNode()
	node.SetBytes(rat)

	metaName, _ := json.Marshal(name)
	node.Node = &tree.Node{
		Uuid:      uuid,
		Type:      nodeType,
		MTime:     mtime,
		Etag:      etag,
		Size:      size,
		Mode:      mode,
		MetaStore: map[string]string{"name": string(metaName)},
	}

	return node, nil
}

func (dao *sqlimpl) checkIntegrity(cat string) error {

	return nil
	ctx := context.Background()
	row := dao.DB.QueryRow(queries["integrity1"])
	var count int
	row.Scan(&count)
	if count > 0 {
		log.Logger(ctx).Error(fmt.Sprintf("[%s] There are %d entries in tree that are not in nodes!", cat, count))
		return nil
	}

	row = dao.DB.QueryRow(queries["integrity2"])
	row.Scan(&count)
	if count > 0 {
		log.Logger(ctx).Debug(fmt.Sprintf("[%s] There are %d entries in tree that are not in tree!", cat, count))
		return nil
	}
	log.Logger(ctx).Debug(fmt.Sprintf("[%s] Integrity test PASSED", cat))
	return nil
}

func (dao *sqlimpl) lock() {
	if current, ok := mu.Load().(*sync.Mutex); ok {
		current.Lock()
	}
}

func (dao *sqlimpl) unlock() {
	if current, ok := mu.Load().(*sync.Mutex); ok {
		current.Unlock()
	}
}

// NewBatchSend Creation of the channels
func NewBatchSend() *BatchSend {
	b := new(BatchSend)
	b.in = make(chan *utils.TreeNode)
	b.out = make(chan error, 1)

	return b
}

// Send a node to the batch
func (b *BatchSend) Send(arg interface{}) {
	if node, ok := arg.(*utils.TreeNode); ok {
		b.in <- node
	}
}

// Close the Batch
func (b *BatchSend) Close() error {
	close(b.in)

	err := <-b.out

	return err
}
