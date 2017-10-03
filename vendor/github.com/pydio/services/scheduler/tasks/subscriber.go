package tasks

import (
	"strconv"
	"strings"
	"sync"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/metadata"
	"github.com/micro/go-micro/server"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// Handle incoming events, apply selectors if any
// and generate all ActionMessage to trigger actions
type Subscriber struct {
	Client          client.Client
	MainQueue       chan Runnable
	UpdateTasksChan chan *jobs.Task

	JobsDefinitions map[string]*jobs.Job
	Dispatchers     map[string]*Dispatcher

	jobsLock    *sync.RWMutex
	RootContext context.Context
}

func NewSubscriber(client client.Client, server server.Server) *Subscriber {
	m := &Subscriber{
		Client:          client,
		JobsDefinitions: make(map[string]*jobs.Job),
		MainQueue:       make(chan Runnable),
		UpdateTasksChan: make(chan *jobs.Task),
		Dispatchers:     make(map[string]*Dispatcher),
		jobsLock:        &sync.RWMutex{},
	}

	m.RootContext = context.WithValue(context.Background(), common.PYDIO_CONTEXT_USER_KEY, "tasks-service")

	server.Subscribe(server.NewSubscriber(common.TOPIC_JOB_CONFIG_EVENT, m.jobsChangeEvent))

	server.Subscribe(server.NewSubscriber(common.TOPIC_TREE_CHANGES, m.nodeEvent))
	server.Subscribe(server.NewSubscriber(common.TOPIC_META_CHANGES, m.nodeEvent))
	server.Subscribe(server.NewSubscriber(common.TOPIC_TIMER_EVENT, m.timerEvent))

	m.ListenToMainQueue()
	m.ListenToTaskChannel()

	return m
}

func (m *Subscriber) Init() error {

	// Load Jobs Definitions
	jobClients := jobs.NewJobServiceClient(common.SERVICE_JOBS, m.Client)
	streamer, e := jobClients.ListJobs(m.RootContext, &jobs.ListJobsRequest{})
	if e != nil {
		return e
	}

	m.jobsLock.Lock()
	defer m.jobsLock.Unlock()
	for {
		resp, er := streamer.Recv()
		if er != nil {
			break
		}
		if resp == nil {
			continue
		}
		m.JobsDefinitions[resp.Job.ID] = resp.Job
		m.GetDispatcherForJob(resp.Job)
	}
	return nil

}

func (m *Subscriber) ListenToMainQueue() {

	go func() {
		for {
			select {
			case runnable := <-m.MainQueue:
				dispatcher := m.GetDispatcherForJob(runnable.Task.Job)
				dispatcher.JobQueue <- runnable
			}
		}
	}()

}

func (m *Subscriber) ListenToTaskChannel() {

	go func() {
		taskClient := jobs.NewJobServiceClient(common.SERVICE_JOBS, m.Client)
		for {
			select {
			case task := <-m.UpdateTasksChan:
				_, e := taskClient.PutTask(m.RootContext, &jobs.PutTaskRequest{Task: task})
				if e != nil {
					log.Logger(m.RootContext).Error("Error while posting task", zap.Error(e))
				}
			}
		}
	}()

}

func (m *Subscriber) GetDispatcherForJob(job *jobs.Job) *Dispatcher {

	if d, exists := m.Dispatchers[job.ID]; exists {
		return d
	}
	maxWorkers := DefaultMaximumWorkers
	if job.MaxConcurrency > 0 {
		maxWorkers = int(job.MaxConcurrency)
	}
	dispatcher := NewDispatcher(maxWorkers)
	m.Dispatchers[job.ID] = dispatcher
	dispatcher.Run()
	return dispatcher

}

func (m *Subscriber) jobsChangeEvent(ctx context.Context, msg *jobs.JobChangeEvent) error {
	m.jobsLock.Lock()
	defer m.jobsLock.Unlock()
	// Update config
	if msg.JobRemoved != "" {
		if _, ok := m.JobsDefinitions[msg.JobRemoved]; ok {
			delete(m.JobsDefinitions, msg.JobRemoved)
		}
		// TODO: Shall we stop everything when changing config?
		if dispatcher, ok := m.Dispatchers[msg.JobRemoved]; ok {
			dispatcher.Stop()
			delete(m.Dispatchers, msg.JobRemoved)
		}
	}
	if msg.JobUpdated != nil {
		m.JobsDefinitions[msg.JobUpdated.ID] = msg.JobUpdated
		// TODO: Shall we stop everything when changing config? Or wait that it's idle for next time?
		if dispatcher, ok := m.Dispatchers[msg.JobUpdated.ID]; ok {
			dispatcher.Stop()
			delete(m.Dispatchers, msg.JobUpdated.ID)
			m.GetDispatcherForJob(msg.JobUpdated)
		}
	}

	return nil
}

func (m *Subscriber) timerEvent(ctx context.Context, event *jobs.JobTriggerEvent) error {
	jobId := event.JobID
	// Load Job Data, build selectors
	m.jobsLock.Lock()
	defer m.jobsLock.Unlock()
	j, ok := m.JobsDefinitions[jobId]
	if !ok {
		// Try to load definition directly for JobsService
		jobClients := jobs.NewJobServiceClient(common.SERVICE_JOBS, m.Client)
		resp, e := jobClients.GetJob(ctx, &jobs.GetJobRequest{JobID: jobId})
		if e != nil || resp.Job == nil {
			return nil
		}
		j = resp.Job
	}
	// Should Run job JOBID w. ActionMessage a
	log.Logger(ctx).Debug("Run Job " + jobId + " on timer event " + event.Schedule.String())
	// This timer event probably comes without user in context at that point
	ctx = context.WithValue(ctx, common.PYDIO_CONTEXT_USER_KEY, "pydio.service.task.user")
	ctx = metadata.NewContext(ctx, metadata.Metadata{common.PYDIO_CONTEXT_USER_KEY: "pydio.service.task.user"})
	ctx = context.WithValue(ctx, common.PYDIO_CONTEXT_USER_KEY, "pydio.service.task.user")
	task := NewTaskFromEvent(ctx, j, event, m.UpdateTasksChan)

	go task.EnqueueRunnables(m.Client, m.MainQueue)

	return nil
}

func (m *Subscriber) nodeEvent(ctx context.Context, event *tree.NodeChangeEvent) error {

	//log.Logger(ctx).Debug("Node Change Event: ", zap.Any("msg", event))
	// Browse listening jobs
	m.jobsLock.Lock()
	defer m.jobsLock.Unlock()
	for jobId, jobData := range m.JobsDefinitions {
		for _, eName := range jobData.EventNames {
			if strings.HasPrefix(eName, "NODE_CHANGE:") {
				nodeChangeValue, e := strconv.ParseInt(strings.TrimPrefix(eName, "NODE_CHANGE:"), 10, 32)
				if e != nil {
					continue
				}
				if event.Type == tree.NodeChangeEvent_EventType(int32(nodeChangeValue)) {
					log.Logger(ctx).Debug("Run Job " + jobId + " on event " + eName)
					task := NewTaskFromEvent(ctx, jobData, event, m.UpdateTasksChan)
					go task.EnqueueRunnables(m.Client, m.MainQueue)
				}
			}
		}
	}
	return nil
}
