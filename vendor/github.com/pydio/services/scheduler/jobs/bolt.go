package jobs

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/jobs"
	"go.uber.org/zap"
)

var (
	// Jobs Configurations
	jobsBucketKey = []byte("jobs")
	// Running tasks
	tasksBucketString = "tasks-"
)

type BoltStore struct {
	db *bolt.DB
}

func NewBoltStore(fileName string) (*BoltStore, error) {

	bs := &BoltStore{}
	db, err := bolt.Open(fileName, 0644, nil)
	if err != nil {
		return nil, err
	}
	er := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(jobsBucketKey)
		if err != nil {
			return err
		}
		return nil
	})
	if er != nil {
		db.Close()
		return nil, er
	}
	bs.db = db
	return bs, nil

}

func (b *BoltStore) Close() {
	b.db.Close()
}

func (s *BoltStore) PutJob(job *jobs.Job) error {

	err := s.db.Update(func(tx *bolt.Tx) error {

		bucket := tx.Bucket(jobsBucketKey)
		if job.Tasks != nil {
			// Do not store that
			job.Tasks = nil
		}
		jsonData, err := json.Marshal(job)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(job.ID), jsonData)

	})
	return err

}

func (s *BoltStore) GetJob(jobId string, withTasks jobs.TaskStatus) (*jobs.Job, error) {

	j := &jobs.Job{}
	e := s.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		bucket := tx.Bucket(jobsBucketKey)
		data := bucket.Get([]byte(jobId))
		if data == nil {
			return errors.NotFound(common.SERVICE_JOBS, "Job ID not found")
		}
		err := json.Unmarshal(data, j)
		if err != nil {
			return errors.InternalServerError(common.SERVICE_JOBS, "Cannot deserialize job")
		}
		if withTasks != jobs.TaskStatus_Unknown {
			j.Tasks = []*jobs.Task{}
			jobTasksBucket := tx.Bucket([]byte(tasksBucketString + jobId))
			if jobTasksBucket != nil {
				j.Tasks = s.tasksToChan(jobTasksBucket, withTasks, nil, j.Tasks)
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return j, nil

}

func (s *BoltStore) DeleteJob(jobID string) error {

	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(jobsBucketKey)
		err := bucket.Delete([]byte(jobID))
		if err == nil {
			jobTasksBucket := tx.Bucket([]byte(tasksBucketString + jobID))
			if jobTasksBucket != nil {
				err = tx.DeleteBucket([]byte(tasksBucketString + jobID))
			}
		}
		if err != nil {
			log.Logger(context.Background()).Error("Error on Job Deletion: ", zap.Error(err))
		}
		return err
	})

}

func (s *BoltStore) ListJobs(owner string, eventsOnly bool, timersOnly bool, withTasks jobs.TaskStatus) (chan *jobs.Job, chan bool, error) {

	res := make(chan *jobs.Job)
	done := make(chan bool)

	go func() {

		s.db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(jobsBucketKey)
			c := bucket.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				j := &jobs.Job{}
				err := json.Unmarshal(v, j)
				if err != nil {
					continue
				}
				if (owner != "" && j.Owner != owner) || (eventsOnly && len(j.EventNames) == 0) || (timersOnly && j.Schedule == nil) {
					continue
				}
				if withTasks != jobs.TaskStatus_Unknown {
					j.Tasks = []*jobs.Task{}
					jobTasksBucket := tx.Bucket([]byte(tasksBucketString + j.ID))
					if jobTasksBucket != nil {
						j.Tasks = s.tasksToChan(jobTasksBucket, withTasks, nil, j.Tasks)
						if len(j.Tasks) == 0 {
							continue
						}
					} else {
						continue
					}
				}
				res <- j
			}
			return nil
		})

		done <- true
		close(done)
	}()

	return res, done, nil
}

func (s *BoltStore) PutTask(task *jobs.Task) error {

	jobId := task.JobID

	return s.db.Update(func(tx *bolt.Tx) error {

		tasksBucket, err := tx.CreateBucketIfNotExists([]byte(tasksBucketString + jobId))
		if err != nil {
			return err
		}
		jsonData, err := json.Marshal(task)
		if err != nil {
			return err
		}
		return tasksBucket.Put([]byte(task.ID), jsonData)

	})

}

func (s *BoltStore) ListTasks(jobId string, taskStatus jobs.TaskStatus) (chan *jobs.Task, chan bool, error) {

	results := make(chan *jobs.Task)
	done := make(chan bool)

	go func() {

		s.db.View(func(tx *bolt.Tx) error {

			if len(jobId) > 0 {
				jobTasksBucket := tx.Bucket([]byte(tasksBucketString + jobId))
				if jobTasksBucket == nil {
					return nil
				}
				s.tasksToChan(jobTasksBucket, taskStatus, results, nil)
			} else {

				tx.ForEach(func(name []byte, b *bolt.Bucket) error {
					if strings.HasPrefix(string(name), tasksBucketString) {
						s.tasksToChan(b, taskStatus, results, nil)
					}
					return nil
				})

			}
			done <- true
			close(done)
			return nil
		})

	}()

	return results, done, nil
}

func (s *BoltStore) tasksToChan(bucket *bolt.Bucket, status jobs.TaskStatus, output chan *jobs.Task, sliceOutput []*jobs.Task) []*jobs.Task {

	bucket.ForEach(func(k, v []byte) error {
		task := &jobs.Task{}
		e := json.Unmarshal(v, task)
		if e == nil {
			if status == jobs.TaskStatus_Any || task.Status == status {
				if output != nil {
					output <- task
				}
				if sliceOutput != nil {
					sliceOutput = append(sliceOutput, task)
				}
			}
		}
		return nil
	})

	return sliceOutput

}
