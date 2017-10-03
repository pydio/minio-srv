package timer

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type EventProducer struct {
	Client    jobs.JobServiceClient
	Waiters   map[string]*ScheduleWaiter
	Context   context.Context
	EventChan chan *jobs.JobTriggerEvent
	StopChan  chan bool
}

func NewEventProducer(c jobs.JobServiceClient) *EventProducer {
	e := &EventProducer{
		Client:    c,
		Waiters:   make(map[string]*ScheduleWaiter),
		StopChan:  make(chan bool, 1),
		EventChan: make(chan *jobs.JobTriggerEvent),
	}

	rootContext := context.Background()
	rootContext = context.WithValue(rootContext, common.PYDIO_CONTEXT_USER_KEY, "scheduler")
	e.Context = rootContext

	go func() {
		for {
			select {
			case event := <-e.EventChan:
				log.Logger(e.Context).Debug("Sending Timer Event", zap.Any("event", event))
				client.Publish(e.Context, client.NewPublication(common.TOPIC_TIMER_EVENT, event))
			case <-e.StopChan:
				return
			}
		}
		close(e.StopChan)
		close(e.EventChan)
	}()

	return e
}

func (e *EventProducer) Start() error {

	// Load all schedules
	streamer, err := e.Client.ListJobs(e.Context, &jobs.ListJobsRequest{TimersOnly: true})
	if err != nil {
		return err
	}
	for {
		resp, er := streamer.Recv()
		if er != nil {
			break
		}
		if resp == nil {
			continue
		}
		log.Logger(e.Context).Info("Registering Job", zap.Any("job", resp.Job))
		e.StartOrUpdateJob(resp.Job)
	}
	return nil
}

func (e *EventProducer) StopAll() {
	for jId, w := range e.Waiters {
		w.Stop()
		delete(e.Waiters, jId)
	}
	e.StopChan <- true
}

func (e *EventProducer) StopWaiter(jobId string) {
	if w, ok := e.Waiters[jobId]; ok {
		w.Stop()
		delete(e.Waiters, jobId)
	}
}

func (e *EventProducer) StartOrUpdateJob(job *jobs.Job) {

	// Stop if already running
	jobId := job.ID
	e.StopWaiter(jobId)

	schedule := job.Schedule
	waiter := NewScheduleWaiter(jobId, schedule, e.EventChan)
	waiter.Start()
	e.Waiters[jobId] = waiter

}
