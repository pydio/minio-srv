package meta

import (
	"sync"
	"sync/atomic"

	"github.com/micro/go-micro/errors"

	"time"

	"fmt"

	"github.com/pydio/services/common"
	"github.com/pydio/services/common/sql"
)

var (
	tableName = "meta"
	queries   = map[string]string{
		"upsert":     `INSERT INTO %s (node_id,namespace,data,author,timestamp,format) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE data=?,author=?,timestamp=?,format=?`,
		"deleteNS":   `DELETE FROM %s WHERE namespace=?`,
		"deleteUuid": `DELETE FROM %s WHERE node_id=?`,
		"select":     `SELECT * FROM %s WHERE node_id=?`,
		"selectAll":  `SELECT * FROM %s LIMIT 0, 500`,
	}
	mu atomic.Value
)

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
			if err := database.Prepare(key, fmt.Sprintf(query, tableName)); err != nil {
				return errors.New("Meta", "Error while preparing statements", 500)
			}
		}
	}

	dao.SQLConn.DB = database.GetConn()
	dao.SQLConn.Stmts = database.GetStmts()

	return nil
}

func (dao *sqlimpl) SetMetadata(nodeId string, metadata map[string]string) (err error) {

	if len(metadata) == 0 {
		// Delete all metadata for node
		dao.GetStmts().Get("deleteUuid").Exec(
			nodeId,
		)
	} else {

		for namespace, data := range metadata {
			json := data
			ns := namespace
			if data == "" {
				// Delete namespace
				dao.GetStmts().Get("deleteNS").Exec(ns)
			} else {
				// Insert or update namespace
				tStamp := time.Now().Unix()

				dao.GetStmts().Get("upsert").Exec(
					nodeId,
					ns,
					json,
					"charles",
					tStamp,
					"json",
					json,
					"charles",
					tStamp,
					"json",
				)
			}
		}
	}

	return nil
}

func (dao *sqlimpl) GetMetadata(nodeId string) (metadata map[string]string, err error) {

	r, err := dao.GetStmts().Get("select").Query(nodeId)
	if err != nil {
		return nil, err
	}
	metadata = make(map[string]string)
	defer r.Close()
	for r.Next() {
		row := struct {
			id        string
			namespace string
			author    string
			timestamp int64
			data      string
			format    string
		}{}
		r.Scan(
			&row.id,
			&row.namespace,
			&row.author,
			&row.timestamp,
			&row.data,
			&row.format,
		)
		metadata[row.namespace] = row.data
	}
	if r.Err() != nil {
		return nil, r.Err()
	}
	if len(metadata) == 0 {
		err = errors.NotFound("metadata-not-found", "Cannot find metadata for node "+nodeId)
		return nil, err
	}
	return metadata, nil

}

func (dao *sqlimpl) ListMetadata(query string) (metaByUuid map[string]map[string]string, err error) {

	r, err := dao.GetStmts().Get("selectAll").Query()
	if err != nil {
		return nil, err
	}
	metaByUuid = make(map[string]map[string]string)

	defer r.Close()
	for r.Next() {
		row := struct {
			id        string
			namespace string
			author    string
			timestamp int64
			data      string
			format    string
		}{}
		r.Scan(
			&row.id,
			&row.namespace,
			&row.author,
			&row.timestamp,
			&row.data,
			&row.format,
		)
		metadata, ok := metaByUuid[row.id]
		if !ok {
			metadata = make(map[string]string)
			metaByUuid[row.id] = metadata
		}
		metadata[row.namespace] = row.data
		metadata[row.namespace] = row.data
	}
	if r.Err() != nil {
		return nil, r.Err()
	}
	return metaByUuid, nil

}
