package jobs

import (
	"github.com/micro/go-micro/client"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

// Implement the JobService API
type JobsHandler struct {
	store DAO
}

//////////////////
// JOBS STORE
/////////////////
func (j *JobsHandler) PutJob(ctx context.Context, request *jobs.PutJobRequest, response *jobs.PutJobResponse) error {
	err := j.store.PutJob(request.Job)
	log.Logger(ctx).Debug("Scheduler PutJob", zap.Any("job", request.Job))
	if err != nil {
		return err
	}
	response.Job = request.Job
	client.Publish(ctx, client.NewPublication(common.TOPIC_JOB_CONFIG_EVENT, &jobs.JobChangeEvent{
		JobUpdated: request.Job,
	}))
	if request.Job.AutoStart {
		client.Publish(ctx, client.NewPublication(common.TOPIC_TIMER_EVENT, &jobs.JobTriggerEvent{
			JobID:  response.Job.ID,
			RunNow: true,
		}))
	}
	return nil
}

func (j *JobsHandler) GetJob(ctx context.Context, request *jobs.GetJobRequest, response *jobs.GetJobResponse) error {
	log.Logger(ctx).Debug("Scheduler GetJob", zap.String("jobId", request.JobID))
	job, err := j.store.GetJob(request.JobID, request.LoadTasks)
	if err != nil {
		return err
	}
	response.Job = job
	return nil
}

func (j *JobsHandler) DeleteJob(ctx context.Context, request *jobs.DeleteJobRequest, response *jobs.DeleteJobResponse) error {
	log.Logger(ctx).Debug("Scheduler DeleteJob", zap.String("jobId", request.JobID))
	err := j.store.DeleteJob(request.JobID)
	if err != nil {
		response.Success = false
		return err
	}
	client.Publish(ctx, client.NewPublication(common.TOPIC_JOB_CONFIG_EVENT, &jobs.JobChangeEvent{
		JobRemoved: request.JobID,
	}))
	response.Success = true
	return nil
}

func (j *JobsHandler) ListJobs(ctx context.Context, request *jobs.ListJobsRequest, streamer jobs.JobService_ListJobsStream) error {

	log.Logger(ctx).Debug("Scheduler ListJobs", zap.Any("req", request))
	defer streamer.Close()

	res, done, err := j.store.ListJobs(request.Owner, request.EventsOnly, request.TimersOnly, request.LoadTasks)
	defer close(res)
	if err != nil {
		return err
	}

	for {
		select {
		case <-done:
			return nil
		case j := <-res:
			streamer.Send(&jobs.ListJobsResponse{Job: j})
		}
	}

	return nil
}

//////////////////
// TASKS STORE
/////////////////
func (j *JobsHandler) PutTask(ctx context.Context, request *jobs.PutTaskRequest, response *jobs.PutTaskResponse) error {

	err := j.store.PutTask(request.Task)
	log.Logger(ctx).Debug("Scheduler PutTask", zap.Any("task", request.Task))
	if err != nil {
		return err
	}
	response.Task = request.Task
	client.Publish(ctx, client.NewPublication(common.TOPIC_JOB_TASK_EVENT, &jobs.TaskChangeEvent{
		TaskUpdated: request.Task,
	}))

	return nil
}

func (j *JobsHandler) PutTaskStream(ctx context.Context, streamer jobs.JobService_PutTaskStreamStream) error {

	defer streamer.Close()

	for {
		request, err := streamer.Recv()
		if request == nil {
			break
		}
		if err != nil {
			return err
		}

		log.Logger(ctx).Debug("PutTaskStream", zap.Any("task", request.Task))
		e := j.store.PutTask(request.Task)
		if e != nil {
			return e
		} else {
			sendErr := streamer.Send(&jobs.PutTaskResponse{Task: request.Task})
			if sendErr != nil {
				return e
			}
		}
	}

	return nil
}

func (j *JobsHandler) ListTasks(ctx context.Context, request *jobs.ListTasksRequest, streamer jobs.JobService_ListTasksStream) error {

	log.Logger(ctx).Debug("Scheduler ListTasks")
	defer streamer.Close()

	res, done, err := j.store.ListTasks(request.JobID, request.Status)
	defer close(res)
	if err != nil {
		return err
	}

	for {
		select {
		case <-done:
			return nil
		case t := <-res:
			streamer.Send(&jobs.ListTasksResponse{Task: t})
		}
	}

	return nil

}
