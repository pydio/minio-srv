package tasks

import (
	"github.com/micro/go-micro/client"
	"github.com/micro/protobuf/ptypes"
	"github.com/pborman/uuid"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/common/proto/tree"
	"golang.org/x/net/context"
	"sync"
	"time"
)

type Task struct {
	*jobs.Job
	context        context.Context
	initialMessage jobs.ActionMessage
	lockedTask     *jobs.Task
	lock           *sync.RWMutex
	saveChannel    chan *jobs.Task
	RC             int
}

func NewTaskFromEvent(ctx context.Context, job *jobs.Job, event interface{}, saveChannel chan *jobs.Task) *Task {
	t := &Task{
		context: ctx,
		Job:     job,
		lockedTask: &jobs.Task{
			ID:          uuid.NewUUID().String(),
			JobID:       job.ID,
			Status:      jobs.TaskStatus_Idle,
			ActionsLogs: []*jobs.ActionLog{},
		},
	}
	t.saveChannel = saveChannel
	t.lock = &sync.RWMutex{}
	t.initialMessage = t.createMessage(event)
	return t
}

func (t *Task) Add(delta int) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.RC = t.RC + delta
}

func (t *Task) Done(delta int) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.RC = t.RC - delta
}

func (t *Task) Save() {
	t.saveChannel <- t.lockedTask
}

func (t *Task) SetStatus(status jobs.TaskStatus) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if status == jobs.TaskStatus_Finished && t.RC > 0 {
		status = jobs.TaskStatus_Running
	}
	t.lockedTask.Status = status
}

func (t *Task) SetProgress(progress float32) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.lockedTask.Progress = progress
}

func (t *Task) SetStartTime(ti time.Time) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.lockedTask.StartTime == 0 {
		t.lockedTask.StartTime = int32(ti.Unix())
	}
}

func (t *Task) SetEndTime(ti time.Time) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.lockedTask.EndTime = int32(ti.Unix())
}

func (t *Task) AppendLog(a jobs.Action, in jobs.ActionMessage, out jobs.ActionMessage) {
	t.lock.Lock()
	defer t.lock.Unlock()
	// Remove unnecessary fields
	// Action
	cleanedAction := a
	cleanedAction.ChainedActions = nil
	// Input
	cleanedInput := in
	cleanedInput.Event = nil
	cleanedInput.OutputChain = nil
	// Output
	cleanedOutput := out
	cleanedOutput.Event = nil
	lastMessage := out.GetLastOutput()
	cleanedOutput.OutputChain = []*jobs.ActionOutput{}
	if lastMessage != nil {
		cleanedOutput.OutputChain = append(cleanedOutput.OutputChain, lastMessage)
	}

	t.lockedTask.ActionsLogs = append(t.lockedTask.ActionsLogs, &jobs.ActionLog{
		Action:        &cleanedAction,
		InputMessage:  &cleanedInput,
		OutputMessage: &cleanedOutput,
	})
}

func (t *Task) createMessage(event interface{}) jobs.ActionMessage {
	initialInput := jobs.ActionMessage{}

	if nodeChange, ok := event.(*tree.NodeChangeEvent); ok {
		any, _ := ptypes.MarshalAny(nodeChange)
		initialInput.Event = any
		if nodeChange.Target != nil {

			initialInput = initialInput.WithNode(nodeChange.Target)

		} else if nodeChange.Source != nil {

			initialInput = initialInput.WithNode(nodeChange.Source)

		}

	} else if triggerEvent, ok := event.(*jobs.JobTriggerEvent); ok {

		any, _ := ptypes.MarshalAny(triggerEvent)
		initialInput.Event = any

	}

	return initialInput
}

func (t *Task) EnqueueRunnables(c client.Client, output chan Runnable) {

	r := RootRunnable(c, t.context, t)
	r.Dispatch(t.initialMessage, t.Actions, output)

}
