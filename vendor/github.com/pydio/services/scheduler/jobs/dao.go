package jobs

import "github.com/pydio/services/common/proto/jobs"

type DAO interface {
	PutJob(job *jobs.Job) error
	GetJob(jobId string, withTasks jobs.TaskStatus) (*jobs.Job, error)
	DeleteJob(jobId string) error
	ListJobs(owner string, eventsOnly bool, timersOnly bool, withTasks jobs.TaskStatus) (chan *jobs.Job, chan bool, error)

	PutTask(task *jobs.Task) error
	ListTasks(jobId string, taskStatus jobs.TaskStatus) (chan *jobs.Task, chan bool, error)
}
