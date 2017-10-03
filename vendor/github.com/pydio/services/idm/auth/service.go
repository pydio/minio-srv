package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/coreos/dex/storage/sql"
	"github.com/go-sql-driver/mysql"
	"github.com/micro/go-micro"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/service/context"
)

func NewAuthService(ctx context.Context) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_AUTH),
	)

	var options []micro.Option

	options = append(options, micro.AfterStart(builder(srv)))

	srv.Init(options...)

	return srv, nil

}

func builder(srv micro.Service) func() error {
	return func() error {
		ctx := srv.Options().Context

		config := servicecontext.GetConfig(ctx)

		log.Logger(ctx).Debug("Config ", zap.Any("config", config))

		dsn, dsnOk := config.Get("dsn").(string)
		if !dsnOk {
			return errors.New("Please set up configuration for DSN and connectors config file for Dex service")
		}

		configDex := config.Get("dex")

		log.Logger(ctx).Info("Configuration", zap.Any("config", configDex), zap.String("dsn", dsn))

		var c Config
		remarshall, _ := json.Marshal(configDex)
		if err := json.Unmarshal(remarshall, &c); err != nil {
			return fmt.Errorf("error parsing config file %s: %v", configDex, err)
		}

		// Parse DSN
		mysqlConf, e := mysql.ParseDSN(dsn)
		if e != nil {
			return e
		}

		sqlConfig := new(sql.MySQL)
		sqlConfig.User = mysqlConf.User
		sqlConfig.Password = mysqlConf.Passwd
		parts := strings.Split(mysqlConf.Addr, ":")
		sqlConfig.Host = parts[0] + ":"
		sqlConfig.Port = parts[1]
		sqlConfig.Database = mysqlConf.DBName

		c.Storage.Config = sqlConfig

		go func() {
			err := serve(c)
			if err != nil {
				log.Logger(context.Background()).Error("Error serving", zap.Error(err))
			}
		}()

		return nil
	}
}
