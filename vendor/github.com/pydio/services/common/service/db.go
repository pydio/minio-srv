package service

import (
	"fmt"

	"go.uber.org/zap"

	"golang.org/x/net/context"

	"github.com/go-sql-driver/mysql"
	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/micro/go-micro/server"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/config"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service/context"
	"github.com/pydio/services/common/sql"
)

var database sql.Provider

// TODO make it a map of databases per service
func newDBProvider(service micro.Service) error {

	name := service.Server().Options().Name

	if name == common.SERVICE_CONFIG {
		return nil
	}

	var options []micro.Option
	ctx := service.Options().Context
	dao := servicecontext.GetDAO(ctx)

	if dao == nil {
		return nil
	}

	// Starting DB Connection for the service
	options = append(options, micro.AfterStart(func() error {

		ctx := service.Options().Context
		config := servicecontext.GetConfig(ctx)

		var err error

		database, err = openDBConnection(config)
		if err != nil {
			log.Logger(ctx).Fatal("Failed to open DB connection", zap.Error(err))
		}

		if database == nil {
			// We have no need for a database
			return nil
		}

		if err = dao.Init(database, config); err != nil {
			log.Logger(ctx).Fatal("Failed to init DB provider", zap.Error(err))
		}

		log.Logger(ctx).Info("DAO initialised", zap.Any("dao", dao))

		return err
	}))

	// Closing the DB Connection for the service
	options = append(options, micro.BeforeStop(func() (err error) {
		if database != nil {
			err = database.CloseConn()
		}
		return
	}))

	options = append(options, micro.WrapClient(NewDAOClientWrapper(&dao)))
	options = append(options, micro.WrapHandler(NewDAOHandlerWrapper(&dao)))
	options = append(options, micro.WrapSubscriber(NewDAOSubscriberWrapper(&dao)))

	service.Init(options...)

	return nil
}

// NewConn to the mysql database
func openDBConnection(config config.Map) (sql.Provider, error) {

	var db *sql.SQLConn

	if config == nil {
		return nil, nil
	}

	dsn, ok := config.Get("dsn").(string)
	if !ok || dsn == "" {
		return nil, nil
	}

	// Try to create the database to ensure it exists
	mysqlConfig, err := mysql.ParseDSN(config.Get("dsn").(string))
	if err != nil {
		return nil, errors.InternalServerError(common.SERVICE_INDEX_, "Error while parsing dsn", err)
	}
	dbName := mysqlConfig.DBName
	mysqlConfig.DBName = ""
	rootDSN := mysqlConfig.FormatDSN()
	if db, err = sql.NewSQLConn("mysql", rootDSN, config); err != nil {
		return nil, errors.InternalServerError(common.SERVICE_INDEX_, "Error while initializing DB connection", err)
	}
	if _, err = db.GetConn().Exec(fmt.Sprintf("create database if not exists %s", dbName)); err != nil {
		return nil, errors.InternalServerError(common.SERVICE_INDEX_, "Error while creating database", err)
	}

	if db, err = sql.NewSQLConn("mysql", dsn, config); err != nil {
		return nil, errors.InternalServerError(common.SERVICE_INDEX_, "Error while initializing DB connection", err)
	}
	if err = db.CreateSchema(); err != nil {
		return nil, errors.InternalServerError(common.SERVICE_INDEX_, "Error while creating schema", err)
	}

	return db, nil
}

type daoWrapper struct {
	dao sql.DAO
	client.Client
}

func (c *daoWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	ctx = servicecontext.WithDAO(ctx, c.dao)
	return c.Client.Call(ctx, req, rsp, opts...)
}

// NewDAOClientWrapper wraps a db connection so it can be accessed by subsequent client wrappers.
func NewDAOClientWrapper(dao *sql.DAO) client.Wrapper {
	return func(c client.Client) client.Client {
		return &daoWrapper{*dao, c}
	}
}

// NewDAOHandlerWrapper wraps a db connection within the handler so it can be accessed by the handler itself.
func NewDAOHandlerWrapper(val *sql.DAO) server.HandlerWrapper {
	return func(h server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			ctx = servicecontext.WithDAO(ctx, *val)
			return h(ctx, req, rsp)
		}
	}
}

// NewDAOSubscriberWrapper wraps a db connection for each subscriber
func NewDAOSubscriberWrapper(val *sql.DAO) server.SubscriberWrapper {
	return func(subscriberFunc server.SubscriberFunc) server.SubscriberFunc {
		return func(ctx context.Context, msg server.Publication) error {
			ctx = servicecontext.WithDAO(ctx, *val)
			return subscriberFunc(ctx, msg)
		}
	}
}
