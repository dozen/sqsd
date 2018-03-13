package sqsd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type MessageConsumer struct {
	Tracker          *JobTracker
	Resource         *Resource
	URL              string
	OnHandleJobEnds  func(jobID string, ok bool, err error)
	OnHandleJobStart func(q *Queue)
	Logger           Logger
}

func NewMessageConsumer(resource *Resource, tracker *JobTracker, logger Logger, url string) *MessageConsumer {
	return &MessageConsumer{
		Tracker:          tracker,
		Resource:         resource,
		URL:              url,
		OnHandleJobStart: func(q *Queue) {},
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
		case q := <-c.Tracker.NextQueue():
			syncWait.Add(1)
			go func() {
				defer syncWait.Done()
				c.HandleJob(ctx, q)
			}()
		}
		if loopEnds {
			break
		}
	}
	c.Logger.Info("MessageConsumer closed.")
}

func (c *MessageConsumer) HandleJob(ctx context.Context, q *Queue) {
	c.OnHandleJobStart(q)
	c.Logger.Debug(fmt.Sprintf("job[%s] HandleJob start.\n", q.ID()))
	ok, err := c.CallWorker(ctx, q)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("job[%s] HandleJob request error: %s\n", q.ID(), err))
	}
	if ok {
		c.Resource.DeleteMessage(q.Msg)
	}
	c.Tracker.Complete(q)
	c.Logger.Debug(fmt.Sprintf("job[%s] HandleJob finished.\n", q.ID()))
	c.OnHandleJobEnds(q.ID(), ok, err)
}

func (c *MessageConsumer) CallWorker(ctx context.Context, q *Queue) (bool, error) {
	req, err := http.NewRequest("POST", c.URL, strings.NewReader(*q.Msg.Body))
	if err != nil {
		return false, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "github.com/taiyoh/sqsd-"+GetVersion())
	req.Header.Set("X-Sqsd-Msgid", q.ID())
	req.Header.Set("X-Sqsd-First-Received-At", q.ReceivedAt.Format("2006-01-02T15:04:05Z0700"))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	statusCode := resp.StatusCode
	defer resp.Body.Close()
	if statusCode != 200 {
		return false, nil
	}

	return true, nil
}
