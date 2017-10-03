package user

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

	errors2 "github.com/micro/go-micro/errors"
)

var (
	queries = map[string]string{
		"AddUser":          `replace into users (login, password, group_path) values (?, ?, ?)`,
		"AddAttribute":     `replace into users_attributes (id, name, value) values (?, ?, ?)`,
		"GetAttributes":    `select name, value from users_attributes where id = ?`,
		"DeleteAttribute":  `delete from users_attributes where id = ? and name = ?`,
		"DeleteAttributes": `delete from users_attributes where id = ?`,
		"DeleteUser":       `delete from users where id = ?`,
	}

	search = `select id, login, password, group_path from users `

	hasher = PydioPW{
		PBKDF2_HASH_ALGORITHM: "sha256",
		PBKDF2_ITERATIONS:     1000,
		PBKDF2_SALT_BYTE_SIZE: 32,
		PBKDF2_HASH_BYTE_SIZE: 24,
		HASH_SECTIONS:         4,
		HASH_ALGORITHM_INDEX:  0,
		HASH_ITERATION_INDEX:  1,
		HASH_SALT_INDEX:       2,
		HASH_PBKDF2_INDEX:     3,
	}
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
				return err
			}
		}
	}

	dao.SQLConn.DB = database.GetConn()
	dao.SQLConn.Stmts = database.GetStmts()

	return nil
}

// Add to the mysql DB
func (dao *sqlimpl) Add(in interface{}) error {

	dao.lock()
	defer dao.unlock()

	user, ok := in.(*idm.User)
	if !ok {
		return errors.New("Wrong type")
	}

	password := hasher.CreateHash(user.Password)

	res, err := dao.GetStmts().Get("AddUser").Exec(
		user.Login,
		password,
		user.GroupPath,
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = fmt.Sprintf("%d", id)

	if len(user.Attributes) > 0 {
		for attr, val := range user.Attributes {
			if _, err := dao.GetStmts().Get("AddAttribute").Exec(
				user.ID,
				attr,
				val,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// Find a user in the DB, and verify that password is correct.
// Password is passed in clear form, hashing method is kept internal to the user service
func (dao *sqlimpl) Bind(userName string, password string) (user *idm.User, e error) {

	dao.lock()
	defer dao.unlock()

	queryString := search + " where login='" + userName + "'"
	queryString += " limit 0,1"

	res, err := dao.DB.Query(queryString)
	if err != nil {
		return nil, err
	}

	defer res.Close()
	for res.Next() {
		user = new(idm.User)
		res.Scan(
			&user.ID,
			&user.Login,
			&user.Password,
			&user.GroupPath,
		)
	}
	if user == nil {

		return nil, errors2.NotFound(common.SERVICE_USER, "Cannot find user "+userName)

	} else {

		hashedPass := user.Password
		// Check password
		valid, _ := hasher.CheckDBKDF2PydioPwd(password, hashedPass)
		if valid {
			user.Password = ""
			return user, nil
		}
		// Check with legacy format (coming from PHP, Salt []byte is built differently)
		valid, _ = hasher.CheckDBKDF2PydioPwd(password, hashedPass, true)
		if valid {
			user.Password = ""
			return user, nil
		}

		return nil, errors2.Forbidden(common.SERVICE_USER, "Password does not match")
	}

}

// Search in the mysql DB
func (dao *sqlimpl) Search(query sql.Enquirer, users *[]interface{}) error {

	dao.lock()
	defer dao.unlock()

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
		user := new(idm.User)
		skipPass := ""
		res.Scan(
			&user.ID,
			&user.Login,
			&skipPass,
			&user.GroupPath,
		)

		user.Attributes = make(map[string]string)

		resAttributes, err := dao.GetStmts().Get("GetAttributes").Query(user.ID)
		if err != nil {
			return err
		}

		defer resAttributes.Close()
		for resAttributes.Next() {
			var name, value string
			resAttributes.Scan(
				&name,
				&value,
			)

			user.Attributes[name] = value
		}

		*users = append(*users, user)
	}

	return nil
}

// Del from the mysql DB
func (dao *sqlimpl) Del(query sql.Enquirer) (int64, error) {

	dao.lock()
	defer dao.unlock()

	whereString := sql.NewDAOQuery(query, new(queryConverter)).String()

	if len(whereString) == 0 || len(strings.Trim(whereString, "()")) == 0 {
		return 0, errors.New("Empty condition for Delete, this is too broad a query!")
	}

	if len(whereString) != 0 {
		whereString = " where " + whereString
	}

	queryString := search + whereString

	res, err := dao.DB.Query(queryString)
	if err != nil {
		return 0, err
	}

	rows := 0

	var ids []string
	for res.Next() {
		user := new(idm.User)
		res.Scan(
			&user.ID,
			&user.Login,
			&user.Password,
			&user.GroupPath,
		)

		ids = append(ids, user.ID)
	}
	res.Close()

	for _, id := range ids {
		if _, err := dao.GetStmts().Get("DeleteAttributes").Exec(id); err != nil {
			return 0, err
		}

		if _, err := dao.GetStmts().Get("DeleteUser").Exec(id); err != nil {
			return 0, err
		}

		rows++
	}

	return int64(rows), nil
}

type queryConverter idm.UserSingleQuery

func (c *queryConverter) Convert(val *any.Any) (string, bool) {

	q := new(idm.UserSingleQuery)

	if err := ptypes.UnmarshalAny(val, q); err != nil {
		return "", false
	}

	var where []string
	if q.Login != "" {
		where = append(where, sql.GetQueryValueFor("login", q.Login))
	}
	if q.Password != "" {
		where = append(where, sql.GetQueryValueFor("password", q.Password))
	}
	if q.GroupPath != "" {
		where = append(where, sql.GetQueryValueFor("group_path", q.GroupPath))
	}

	return strings.Join(where, " and "), true
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
