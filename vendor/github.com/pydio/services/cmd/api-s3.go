package cmd

import (
	"context"
	"sync"

	"github.com/micro/cli"
	minio "github.com/pydio/minio-priv/cmd"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/data/source/objects"
	"go.uber.org/zap"
)

var (
	apiS3Flags = []cli.Flag{}

	apiS3Cmd = cli.Command{
		Name:   "s3",
		Usage:  "Starts S3 gateway to access tree service, on port 9020",
		Flags:  apiS3Flags,
		Action: mainApiS3,
	}
)

func mainApiS3(ctx *cli.Context) {

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := startMinioGateway(); err != nil {
			log.Logger(context.Background()).Fatal("Error running s3 gateway", zap.Error(err))
		}
	}()
	wg.Wait()

}

// StartMinioServer handler
func startMinioGateway() error {

	if gatewayDir, e := objects.CreateMinioConfigFile("gateway", "gateway", "gatewaysecret"); e == nil {
		params := []string{"minio", "gateway", "pydio", "--address", ":9020", "--config-dir", gatewayDir /*, "--quiet"*/}
		minio.Main(params)
		return nil
	} else {
		return e
	}

}
