package timer

import (
	"strconv"
	"strings"
	"time"

	"github.com/ajvb/kala/utils/iso8601"
	"github.com/micro/go-micro/errors"
	"github.com/pydio/services/common"
	"github.com/pydio/services/common/proto/jobs"
)

func NewScheduleWaiter(jobId string, schedule *jobs.Schedule, tickerChannel chan *jobs.JobTriggerEvent) *ScheduleWaiter {
	waiter := &ScheduleWaiter{}
	waiter.Schedule = schedule
	waiter.ParseSchedule()
	waiter.jobId = jobId
	waiter.ticker = tickerChannel
	return waiter
}

type ScheduleWaiter struct {
	*jobs.Schedule
	jobId    string
	ticker   chan *jobs.JobTriggerEvent
	stopChan chan bool

	// Number of repetitions: if 0, infinite repetition.
	repeat int64
	// Do not start until that time
	startTime time.Time
	// Interval between each repetition
	interval time.Duration

	lastTick time.Time
}

func (w *ScheduleWaiter) Start() {
	w.stopChan = make(chan bool)
	w.WaitUntilNext()
}

func (w *ScheduleWaiter) Stop() {
	w.stopChan <- true
}

func (w *ScheduleWaiter) WaitUntilNext() {

	now := time.Now()
	var wait time.Duration
	// First let's wait until start time
	if wait = w.startTime.Sub(now); wait < 0 {
		// Start time is behind us, let's have a look at intervals now
		if w.lastTick.IsZero() {
			wait = 0 // Start now ?
		} else {
			wait = w.lastTick.Add(w.interval).Sub(now) // Compute next run and diff
		}
	}

	go func() {
		for {
			select {
			case <-time.After(wait):
				w.ticker <- &jobs.JobTriggerEvent{
					JobID:    w.jobId,
					Schedule: w.Schedule,
				}
				w.lastTick = time.Now()
				w.WaitUntilNext()
				return
			case <-w.stopChan:
				return
			}
		}
	}()

}

func (w *ScheduleWaiter) ParseSchedule() error {

	parts := strings.Split(w.Iso8601Schedule, "/")
	if len(parts) != 3 {
		return errors.InternalServerError(common.SERVICE_TIMER, "Invalid format for schedule")
	}
	repeatString := parts[0]
	startString := parts[1]
	intervalString := parts[2]

	if repeatString != "R" {
		w.repeat, _ = strconv.ParseInt(strings.TrimPrefix(parts[0], "R"), 10, 64)
	}

	var err error
	w.startTime, err = time.Parse(time.RFC3339, startString)
	if err != nil {
		w.startTime, err = time.Parse("2006-01-02T15:04:05", startString)
		if err != nil {
			return err
		}
	}

	if w.repeat > 1 || w.repeat == 0 {
		isoDuration, er := iso8601.FromString(intervalString)
		if er != nil {
			return err
		}
		w.interval = isoDuration.ToDuration()
	}

	return nil
}
