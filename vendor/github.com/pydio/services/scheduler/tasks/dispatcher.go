package tasks

const (
	DefaultMaximumWorkers = 20
)

type Dispatcher struct {
	// A pool of workers channels that are registered with the dispatcher
	JobQueue   chan Runnable
	WorkerPool chan chan Runnable
	maxWorker  int
}

func NewDispatcher(maxWorkers int) *Dispatcher {
	pool := make(chan chan Runnable, maxWorkers)
	jobQueue := make(chan Runnable)
	return &Dispatcher{
		WorkerPool: pool,
		maxWorker:  maxWorkers,
		JobQueue:   jobQueue,
	}
}

func (d *Dispatcher) Run() {
	// starting n number of workers
	for i := 0; i < d.maxWorker; i++ {
		worker := NewWorker(d.WorkerPool, d.JobQueue)
		worker.Start()
	}

	go d.dispatch()
}

func (d *Dispatcher) Stop() {
	// TODO
	// Use a signal to send Quit to all workers
}

func (d *Dispatcher) dispatch() {
	for {
		select {
		case jobImpl := <-d.JobQueue:
			// a jobs request has been received
			go func(job Runnable) {
				// try to obtain a worker job channel that is available.
				// this will block until a worker is idle
				jobChannel := <-d.WorkerPool

				// dispatch the job to the worker job channel
				jobChannel <- job
			}(jobImpl)
		}
	}
}
