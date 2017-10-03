package task

import (
	"log"
	"time"

	. "github.com/pydio/poc/sync/common"
	"github.com/pydio/poc/sync/filters"
	"github.com/pydio/poc/sync/proc"
)

type Sync struct {
	Source Endpoint
	Target Endpoint

	EchoFilter *filters.EchoFilter
	Merger     *proc.Merger
	Direction  string

	doneChans []chan bool
}

func (s *Sync) SetupWatcher(source PathSyncSource, target PathSyncTarget) error {

	var err error
	watchObject, err := source.Watch("")
	if err != nil {
		return err
	}

	s.doneChans = append(s.doneChans, watchObject.DoneChan)

	// Now wire batches to processor
	batcher := filters.NewEventsBatcher(source, target)

	filterIn, filterOut := s.EchoFilter.CreateFilter()
	s.Merger.AddRequeueChannel(source, filterIn)
	go batcher.BatchEvents(filterOut, s.Merger.BatchesChannel, 1*time.Second)

	go func() {

		// Wait for all events.
		for {
			select {
			case event, ok := <-watchObject.Events():
				if !ok {
					continue
				}
				//log.Printf(" WATCH EVENT| %v %v %v %v", event.Type, event.Path, event.Time, event.Size)
				filterIn <- event
			case err, ok := <-watchObject.Errors():
				if !ok {
					continue
				}
				if err != nil {
					log.Println(err, ok)
				}
			}
		}
	}()

	return nil

}

func (s *Sync) InitialSnapshots(dryRun bool) (diff *proc.SourceDiff, e error) {

	source, _ := AsPathSyncSource(s.Source)
	targetAsSource, tASOk := AsPathSyncSource(s.Target)
	diff, e = proc.ComputeSourcesDiff(source, targetAsSource, dryRun)

	if e != nil {
		return nil, e
	}
	if dryRun {
		return diff, nil
	}

	//log.Println("Initial Snapshot Diff:", diff)
	var batchLeft, batchRight *filters.Batch
	var err error
	// Changes must be applied from Source to Target only
	if s.Direction == "left" {
		batchLeft, err = diff.ToUnidirectionalBatch("left")
		if err != nil {
			batchLeft = &filters.Batch{}
		}
		batchRight = &filters.Batch{}
	} else if s.Direction == "right" {
		batchLeft = &filters.Batch{}
		batchRight, err = diff.ToUnidirectionalBatch("right")
		if err != nil {
			batchRight = &filters.Batch{}
		}
	} else {
		sourceAsTarget, _ := AsPathSyncTarget(s.Source)
		target, _ := AsPathSyncTarget(s.Target)
		biBatch, err := diff.ToBidirectionalBatch(sourceAsTarget, target)
		if err == nil {
			batchLeft = biBatch.Left
			batchRight = biBatch.Right
		}
	}

	if sTarget, ok := AsPathSyncTarget(s.Target); ok {
		leftBatchFilter := filters.NewEventsBatcher(source, sTarget)
		leftBatchFilter.FilterBatch(batchLeft)
	}

	if sTarget2, ok2 := AsPathSyncTarget(s.Source); ok2 && tASOk {
		rightBatchFilter := filters.NewEventsBatcher(targetAsSource, sTarget2)
		rightBatchFilter.FilterBatch(batchRight)
	}

	log.Printf("Initial Snapshot Batch:\n --LEFT \n%v \n -- RIGHT \n %v", batchLeft, batchRight)

	s.Merger.BatchesChannel <- batchLeft
	s.Merger.BatchesChannel <- batchRight

	return diff, nil
}

func (s *Sync) Shutdown() {
	for _, channel := range s.doneChans {
		close(channel)
	}
	s.Merger.Shutdown()
}

func (s *Sync) Start() {
	source, sOk := AsPathSyncSource(s.Source)
	target, tOk := AsPathSyncTarget(s.Target)
	if s.Direction != "right" && sOk && tOk {
		s.SetupWatcher(source, target)
	}
	source2, sOk2 := AsPathSyncSource(s.Target)
	target2, tOk2 := AsPathSyncTarget(s.Source)
	if s.Direction != "left" && sOk2 && tOk2 {
		s.SetupWatcher(source2, target2)
	}
}

func (s *Sync) Resync(dryRun bool) (*proc.SourceDiff, error) {
	return s.InitialSnapshots(dryRun)
}

func NewSync(left Endpoint, right Endpoint) *Sync {

	filter := filters.NewEchoFilter()
	merger := proc.NewMerger()
	merger.LocksChannel = filter.LockEvents
	merger.UnlocksChannel = filter.UnlockEvents
	go merger.ProcessBatches()
	go filter.ListenLocksEvents()

	return &Sync{
		Source: left,
		Target: right,

		EchoFilter: filter,
		Merger:     merger,
	}

}
