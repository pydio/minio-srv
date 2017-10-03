package tasks

import (
	"github.com/pydio/services/common/log"
)

// Worker represents the worker that executes the jobs
type Worker struct {
	WorkerPool chan chan Runnable
	JobChannel chan Runnable
	quit       chan bool
	JobReQueue chan Runnable
}

func NewWorker(workerPool chan chan Runnable, requeue chan Runnable) Worker {
	return Worker{
		WorkerPool: workerPool,
		JobChannel: make(chan Runnable),
		JobReQueue: requeue,
		quit:       make(chan bool)}
}

// Start method starts the run loop for the worker, listening for a quit channel in
// case we need to stop it
func (w Worker) Start() {
	go func() {
		for {
			// register the current worker into the worker queue.
			w.WorkerPool <- w.JobChannel

			select {
			case runnable := <-w.JobChannel:
				// we have received a work request.
				//log.Logger(runnable.Context).Debug("Received Runnable in dispatcher", zap.String("runnable", runnable.ID))
				err := runnable.RunAction(w.JobReQueue)
				// Todo : do something with errors
				if err != nil {
					log.Logger(runnable.Context).Error(err.Error())
				}
				//log.Logger(runnable.Context).Debug("Runnable in dispatcher: Finished", zap.String("runnable", runnable.ID))

			case <-w.quit:
				// we have received a signal to stop
				return
			}
		}
	}()
}

// Stop signals the worker to stop listening for work requests.
func (w Worker) Stop() {
	go func() {
		w.quit <- true
	}()
}
