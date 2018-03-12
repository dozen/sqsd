package sqsd

import (
	"context"
	"fmt"
	"sync"
)

type MessageConsumer struct {
	Tracker          *JobTracker
	Resource         *Resource
	OnHandleJobEnds  func(jobID string, ok bool, err error)
	OnHandleJobStart func(job *Job)
	Logger           Logger
}

func NewMessageConsumer(resource *Resource, tracker *JobTracker, logger Logger) *MessageConsumer {
	return &MessageConsumer{
		Tracker:          tracker,
		Resource:         resource,
		OnHandleJobStart: func(job *Job) {},
		OnHandleJobEnds:  func(jobID string, ok bool, err error) {},
		Logger:           logger,
	}
}

func (c *MessageConsumer) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	syncWait := new(sync.WaitGroup)
	loopEnds := false
	c.Logger.Info("MessageConsumer start.")
	for {
		select {
		case <-ctx.Done():
			syncWait.Wait()
			loopEnds = true
			break
		case job := <-c.Tracker.NextJob():
			syncWait.Add(1)
			go func() {
				defer syncWait.Done()
				c.HandleJob(ctx, job)
			}()
		}
		if loopEnds {
			break
		}
	}
	c.Logger.Info("MessageConsumer closed.")
}

func (c *MessageConsumer) HandleJob(ctx context.Context, job *Job) {
	c.OnHandleJobStart(job)
	c.Logger.Debug(fmt.Sprintf("job[%s] HandleJob start.\n", job.ID()))
	ok, err := job.Run(ctx)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("job[%s] HandleJob request error: %s\n", job.ID(), err))
	}
	if ok {
		c.Resource.DeleteMessage(job.Msg)
	}
	c.Tracker.Complete(job)
	c.Logger.Debug(fmt.Sprintf("job[%s] HandleJob finished.\n", job.ID()))
	c.OnHandleJobEnds(job.ID(), ok, err)
}
