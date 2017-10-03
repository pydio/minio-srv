package sql

import (
	"database/sql"
	"fmt"

	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
)

var (
	ErrNoRows = sql.ErrNoRows
)

// Connection to the DB wrapper around sql.DB with Stmts and Options
type SQLConn struct {
	DB *sql.DB

	Stmts   Stmts
	Options common.Manager
}

type Rows sql.Rows

// Open a connection to the database
func NewSQLConn(driver string, dsn string, optionsArr ...common.Manager) (*SQLConn, error) {

	var connection *sql.DB
	var err error
	var options common.Manager

	if connection, err = sql.Open(driver, dsn); err != nil {
		return nil, err
	}

	return &SQLConn{
		DB:      connection,
		Options: options,
		Stmts:   Stmts{config.NewMap()},
	}, nil
}

// GetConn of this instance
func (db *SQLConn) GetConn() *sql.DB {
	return db.DB
}

// ResetConn of this instance
func (db *SQLConn) ResetConn(conn *sql.DB) {
	db.DB = conn
}

// CloseConn for this instance
func (db *SQLConn) CloseConn() error {
	if db.DB != nil {
		return db.DB.Close()
	}

	return nil
}

// CreateSchema in the Database
func (db *SQLConn) CreateSchema(schema ...string) error {
	for _, instruction := range schema {
		if _, err := db.DB.Exec(instruction); err != nil {
			return fmt.Errorf("Failed to prepare database: %s", err)
		}
	}
	return nil
}

// Prepare Statements for the SQL database
func (db *SQLConn) Prepare(key string, query string, tables ...interface{}) error {

	query = fmt.Sprintf(query, tables...)

	stmt, err := db.DB.Prepare(query)
	if err != nil {
		return fmt.Errorf("Preparing statement : %v, %v", query, err)
	}

	db.Stmts.Set(key, stmt)

	return nil
}

// GetStmts for this DB Connection
func (db *SQLConn) GetStmts() Stmts {
	return db.Stmts
}

// GetOptions for this DB Connection
func (db *SQLConn) GetOptions() common.Manager {
	return db.Options
}
