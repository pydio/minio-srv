package tasks

import (
	"time"

	"github.com/micro/go-micro/client"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/jobs"
	"github.com/pydio/services/scheduler/actions"
	"golang.org/x/net/context"
)

type Runnable struct {
	jobs.Action
	Task           *Task
	Message        jobs.ActionMessage
	Client         client.Client
	Context        context.Context
	Implementation actions.ConcreteAction
}

func RootRunnable(cl client.Client, ctx context.Context, task *Task) Runnable {
	return Runnable{
		Context: ctx,
		Client:  cl,
		Task:    task,
	}
}

func NewRunnable(cl client.Client, ctx context.Context, task *Task, action *jobs.Action, message jobs.ActionMessage) Runnable {
	r := Runnable{
		Action:  *action,
		Task:    task,
		Client:  cl,
		Context: ctx,
		Message: message,
	}
	// Find Concrete Implementation from ActionID
	impl, ok := actions.GetActionsManager().ActionById(action.ID)
	if ok {
		r.Implementation = impl
		r.Implementation.Init(task.Job, cl, action)
	}
	return r
}

// Replicate runnable for child action
func (r *Runnable) CreateChild(action *jobs.Action, message jobs.ActionMessage) Runnable {

	r.Task.Add(1)
	return NewRunnable(r.Client, r.Context, r.Task, action, message)

}

func (r *Runnable) Dispatch(input jobs.ActionMessage, actions []*jobs.Action, Queue chan Runnable) {

	for _, action := range actions {
		act := action
		messagesOutput := make(chan jobs.ActionMessage)
		go func() {
			for {
				select {
				case message := <-messagesOutput:
					// Build runnable and enqueue
					Queue <- r.CreateChild(act, message)
				}
			}
		}()
		action.ToMessages(input, r.Client, r.Context, messagesOutput)
	}

}

func (r *Runnable) RunAction(Queue chan Runnable) error {

	if r.Implementation == nil {
		return errors.NotFound(common.SERVICE_JOBS, "Error while running action: no Concrete Action found")
	}

	r.Task.SetStatus(jobs.TaskStatus_Running)
	r.Task.SetStartTime(time.Now())
	r.Task.Save()
	outputMessage, err := r.Implementation.Run(r.Context, r.Message)
	r.Task.Done(1)
	if err != nil {
		r.Task.SetStatus(jobs.TaskStatus_Error)
		r.Task.SetEndTime(time.Now())
		r.Task.Save()
		return err
	}
	r.Task.AppendLog(r.Action, r.Message, outputMessage)

	r.Dispatch(outputMessage, r.ChainedActions, Queue)

	r.Task.SetStatus(jobs.TaskStatus_Finished)
	r.Task.SetEndTime(time.Now())
	r.Task.Save()

	return nil
}
