package sql

import (
	"database/sql"

	"github.com/pydio/services/common"
)

// DAO interface definition
type DAO interface {
	Init(Provider, common.Manager) error
	Provider
}

// Provider of a DB Connection interface
type Provider interface {
	GetConn() *sql.DB
	ResetConn(*sql.DB)
	CloseConn() error
	CreateSchema(...string) error
	Prepare(string, string, ...interface{}) error
	GetStmts() Stmts
	GetOptions() common.Manager
}

// Stmts definition
type Stmts struct {
	common.Manager
}

// Get a specific statement
func (s Stmts) Get(key string) *sql.Stmt {
	if v, ok := s.Manager.Get(key).(*sql.Stmt); ok {
		return v
	}
	return nil
}
