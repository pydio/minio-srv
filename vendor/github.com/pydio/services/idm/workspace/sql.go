package workspace

import (
	"errors"
	"sync"

	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/idm"
	"github.com/pydio/services/common/sql"
)

var (
	queries = map[string]string{
		"AddWorkspace": `insert into workspaces (uuid, label, slug) values (?, ?, ?)`,
		"GetWorkspace": `select uuid from workspaces where uuid = ?`,
	}

	search = `select * from workspaces`
	delete = `delete from workspaces`
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
			if err := database.Prepare(key, query); err != nil {
				return errors.New("Error while preparing statements")
			}
		}
	}

	dao.SQLConn.DB = database.GetConn()
	dao.SQLConn.Stmts = database.GetStmts()

	return nil
}

// Add to the mysql DB
func (dao *sqlimpl) Add(in interface{}) error {

	workspace, ok := in.(*idm.Workspace)
	if !ok {
		return errors.New("Wrong type")
	}

	res, err := dao.GetStmts().Get("AddWorkspace").Exec(workspace.UUID, workspace.Label, workspace.Slug)
	if err == nil {
		rows, err2 := res.RowsAffected()
		if err2 != nil {
			return err2
		}

		if rows > 0 {
			uuid, err3 := res.LastInsertId()
			if err3 != nil {
				return err3
			}

			workspace.UUID = fmt.Sprintf("%d", uuid)

			return nil
		}
	}

	row := dao.GetStmts().Get("GetWorkspace").QueryRow(workspace.Label)
	if row == nil {
		return err
	}

	var uuid string
	row.Scan(&uuid)

	workspace.UUID = uuid

	return nil
}

// Search in the mysql DB
func (dao *sqlimpl) Search(query sql.Enquirer, workspaces *[]interface{}) error {

	whereString := sql.NewDAOQuery(query, new(queryConverter)).String()

	//	whereString, _ := query.Build(new(queryConverter))

	if len(whereString) != 0 {
		whereString = " where " + whereString
	}

	offset, limit := int64(0), int64(0)
	if query.GetOffset() > 0 {
		offset = query.GetOffset()
	}
	if query.GetLimit() == 0 {
		// Default limit
		limit = 100
	}

	limitString := fmt.Sprintf(" limit %v,%v", offset, limit)
	if query.GetLimit() == -1 {
		limitString = ""
	}

	queryString := search + whereString + limitString

	res, err := dao.DB.Query(queryString)
	if err != nil {
		return err
	}

	defer res.Close()
	for res.Next() {
		workspace := new(idm.Workspace)
		res.Scan(
			&workspace.UUID,
			&workspace.Label,
			&workspace.Slug,
		)

		*workspaces = append(*workspaces, workspace)
	}

	return nil
}

// Del from the mysql DB
func (dao *sqlimpl) Del(query sql.Enquirer) (int64, error) {

	whereString := sql.NewDAOQuery(query, new(queryConverter)).String()

	if len(whereString) == 0 || len(strings.Trim(whereString, "()")) == 0 {
		return 0, errors.New("Empty condition for Delete, this is too broad a query!")
	}

	if len(whereString) != 0 {
		whereString = " where " + whereString
	}

	queryString := delete + whereString

	res, err := dao.DB.Exec(queryString)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rows, nil
}

type queryConverter idm.WorkspaceSingleQuery

func (c *queryConverter) Convert(val *any.Any) (string, bool) {

	q := new(idm.WorkspaceSingleQuery)

	if err := ptypes.UnmarshalAny(val, q); err != nil {
		return "", false
	}

	return sql.GetQueryValueFor("uuid", q.Uuid), true
}
