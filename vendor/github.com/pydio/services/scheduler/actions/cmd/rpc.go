package cmd

import (
	"encoding/json"
	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/jobs"
	"golang.org/x/net/context"
)

type RpcAction struct {
	Client      client.Client
	ServiceName string
	MethodName  string
	JsonRequest interface{}
}

var (
	rpcActionName = "actions.cmd.rpc"
)

// Unique identifier
func (c *RpcAction) GetName() string {
	return rpcActionName
}

// Pass parameters
func (c *RpcAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {
	c.Client = cl
	c.ServiceName = action.Parameters["service"]
	c.MethodName = action.Parameters["method"]
	if c.ServiceName == "" || c.MethodName == "" {
		return errors.BadRequest(common.SERVICE_JOBS, "Missing parameters for RPC Action")
	}
	if jsonParams, o := action.Parameters["request"]; o {
		var jsonData interface{}
		e := json.Unmarshal([]byte(jsonParams), &jsonData)
		if e == nil {
			c.JsonRequest = jsonData
		}
	}
	return nil
}

// Run the actual action code
func (c *RpcAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {

	req := c.Client.NewJsonRequest(c.ServiceName, c.MethodName, c.JsonRequest)
	var response json.RawMessage
	e := c.Client.Call(ctx, req, &response)
	if e != nil {
		return input.WithError(e), e
	}
	output := input
	jsonData, _ := response.MarshalJSON()
	output.AppendOutput(&jobs.ActionOutput{
		Success:  true,
		JsonBody: jsonData,
	})
	return output, nil

}
