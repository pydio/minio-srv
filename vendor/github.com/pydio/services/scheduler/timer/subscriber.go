package timer

import (
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type JobsEventsSubscriber struct {
	Producer *EventProducer
}

func (e *JobsEventsSubscriber) Handle(ctx context.Context, msg *jobs.JobChangeEvent) error {

	log.Logger(ctx).Debug("JobsEvent Subscriber", zap.Any("event", msg))

	if msg.JobRemoved != "" {
		e.Producer.StopWaiter(msg.JobRemoved)
	}
	if msg.JobUpdated != nil && msg.JobUpdated.Schedule != nil {
		e.Producer.StartOrUpdateJob(msg.JobUpdated)
	}
	return nil

}
