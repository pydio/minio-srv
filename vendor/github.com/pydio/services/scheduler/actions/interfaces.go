package actions

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/proto/jobs"
	"golang.org/x/net/context"
)

type Concrete func() ConcreteAction

type ConcreteAction interface {

	// Unique identifier
	GetName() string
	// Pass parameters
	Init(job *jobs.Job, cl client.Client, action *jobs.Action) error
	// Run the actual action code
	Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error)
}
