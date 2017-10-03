package cmd

import (
	"context"

	"github.com/micro/cli"
	"github.com/pydio/services/broker/websocket"
	"github.com/pydio/services/common/log"
	"go.uber.org/zap"
)

var (
	apiWebSocketFlags = []cli.Flag{
		cli.IntFlag{
			Name:  "ws_port",
			Usage: "Port for WebSocket server (5050 by default). Open localhost:5050/test.html to check events feed.",
			Value: 5050,
		},
	}

	apiWebSocketCmd = cli.Command{
		Name:   "ws",
		Usage:  "Starts WebSocket service on port 5000",
		Flags:  apiWebSocketFlags,
		Action: mainApiWebSocket,
	}
)

func mainApiWebSocket(ctx *cli.Context) {

	port := 5050
	if ctx.Int("ws_port") > 0 {
		port = ctx.Int("ws_port")
	}
	serv, err := websocket.NewWebSocketService(port)

	if err != nil {
		log.Logger(context.Background()).Fatal("Error creating websocket", zap.Error(err))
	}

	if err := serv.Run(); err != nil {
		log.Logger(context.Background()).Fatal("Error running websocket", zap.Error(err))
	}

}
