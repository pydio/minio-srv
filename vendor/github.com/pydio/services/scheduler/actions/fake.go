package actions

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"time"
)

func init() {
	GetActionsManager().Register("FAKE", func() ConcreteAction {
		return &FakeAction{}
	})
}

type FakeAction struct{}

// Unique identifier
func (f *FakeAction) GetName() string {
	return "FAKE"
}

// Pass parameters
func (f *FakeAction) Init(job *jobs.Job, cl client.Client, action *jobs.Action) error {
	return nil
}

// Run the actual action code
func (f *FakeAction) Run(ctx context.Context, input jobs.ActionMessage) (jobs.ActionMessage, error) {
	outputMessage := input
	outputMessage.AppendOutput(&jobs.ActionOutput{StringBody: "Hello World"})
	time.Sleep(time.Second * 4)
	log.Logger(ctx).Debug("End Fake Task", zap.Any("output", outputMessage))
	return outputMessage, nil
}
