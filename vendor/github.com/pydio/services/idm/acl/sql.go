package acl

import (
	"errors"
	"sync"

	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/acl"
	"github.com/pydio/services/common/sql"
)

var (
	queries = map[string]string{
		"AddACL":          `insert into acls (action_name, action_value, role_id, workspace_id, node_id) values (?, ?, ?, ?, ?)`,
		"AddACLNode":      `insert into acl_nodes (uuid) values (?)`,
		"AddACLWorkspace": `insert into acl_workspaces (name) values (?)`,
		"GetACLNode":      `select id from acl_nodes where uuid = ?`,
		"GetACLWorkspace": `select id from acl_workspaces where name = ?`,
	}

	search = `select a.id, n.uuid, a.action_name, a.action_value, a.role_id, w.name from acls a, acl_nodes n, acl_workspaces w where n.id = a.node_id and w.id = a.workspace_id `
	delete = `delete from acls`
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

	val, ok := in.(*acl.ACL)
	if !ok {
		return errors.New("Wrong type")
	}

	if val.Action == nil {
		return errors.New("Missing action value")
	}

	workspaceID := "-1"
	if val.WorkspaceID != "" {
		id, err := dao.addWorkspace(val.WorkspaceID)
		if err != nil {
			return err
		}
		workspaceID = id
	}

	nodeID := "-1"
	if val.NodeID != "" {
		id, err := dao.addNode(val.NodeID)
		if err != nil {
			return err
		}
		nodeID = id
	}

	res, err := dao.GetStmts().Get("AddACL").Exec(val.Action.Name, val.Action.Value, val.RoleID, workspaceID, nodeID)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	val.ID = fmt.Sprintf("%d", id)

	return nil
}

// Search in the mysql DB
func (dao *sqlimpl) Search(query sql.Enquirer, acls *[]interface{}) error {

	whereString := sql.NewDAOQuery(query, new(queryConverter)).String()

	//	whereString, _ := query.Build(new(queryConverter))

	if len(whereString) != 0 {
		whereString = " and " + whereString
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
		val := new(acl.ACL)
		action := new(acl.ACLAction)

		res.Scan(
			&val.ID,
			&val.NodeID,
			&action.Name,
			&action.Value,
			&val.RoleID,
			&val.WorkspaceID,
		)

		val.Action = action

		*acls = append(*acls, val)
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

func (dao *sqlimpl) addWorkspace(uuid string) (string, error) {

	res, err := dao.GetStmts().Get("AddACLWorkspace").Exec(uuid)
	if err == nil {
		rows, err := res.RowsAffected()
		if err != nil {
			return "", err
		}

		if rows > 0 {
			id, err := res.LastInsertId()
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("%d", id), nil
		}
	}

	row := dao.GetStmts().Get("GetACLWorkspace").QueryRow(uuid)
	if row == nil {
		return "", err
	}

	var id string
	row.Scan(&id)

	return id, nil
}

func (dao *sqlimpl) addNode(uuid string) (string, error) {

	res, err := dao.GetStmts().Get("AddACLNode").Exec(uuid)
	if err == nil {
		rows, err := res.RowsAffected()
		if err != nil {
			return "", err
		}

		if rows > 0 {
			id, err := res.LastInsertId()
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("%d", id), nil
		}
	}

	// Checking we didn't have a duplicate
	row := dao.GetStmts().Get("GetACLNode").QueryRow(uuid)
	if row == nil {
		return "", err
	}

	var id string
	row.Scan(&id)

	return id, nil
}

type queryConverter acl.ACLSingleQuery

func (c *queryConverter) Convert(val *any.Any) (string, bool) {

	q := new(acl.ACLSingleQuery)

	if err := ptypes.UnmarshalAny(val, q); err != nil {
		return "", false
	}

	var wheres []string

	if len(q.RoleIDs) > 0 {
		wheres = append(wheres, sql.GetQueryValueFor("role_id", q.RoleIDs...))
	}

	if len(q.WorkspaceIDs) > 0 {
		wheres = append(wheres, fmt.Sprintf("workspace_id in (select id from acl_workspaces where name in (%s))", strings.Join(Map(q.WorkspaceIDs, Quote), ",")))
	}

	if len(q.NodeIDs) > 0 {
		wheres = append(wheres, fmt.Sprintf("node_id in (select id from acl_nodes where uuid in (%s))", strings.Join(Map(q.NodeIDs, Quote), ",")))
	}

	// Special case for Actions
	if len(q.Actions) > 0 {
		actionsByName := make(map[string][]string) // actionName => actionValues
		for _, act := range q.Actions {
			values, exists := actionsByName[act.Name]
			if !exists {
				values = []string{}
				actionsByName[act.Name] = values
			}
			if act.Value != "" {
				values = append(values, act.Value)
			}
		}
		var orWheres []string
		for actName, actValues := range actionsByName {
			var actionWheres []string

			actionWheres = append(actionWheres, sql.GetQueryValueFor("action_name", actName))
			if len(actValues) > 0 {
				actionWheres = append(actionWheres, sql.GetQueryValueFor("action_value", actValues...))
				orWheres = append(orWheres, "("+strings.Join(actionWheres, " AND ")+")")
			} else {
				orWheres = append(orWheres, strings.Join(actionWheres, ""))
			}
		}
		if len(orWheres) > 1 {
			wheres = append(wheres, strings.Join(orWheres, " OR "))
		} else {
			wheres = append(wheres, strings.Join(orWheres, ""))
		}
	}

	return strings.Join(wheres, " AND "), true
}

func Quote(v string) string {
	return fmt.Sprintf(`"%s"`, v)
}

func Map(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}
