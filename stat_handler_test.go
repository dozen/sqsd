package sqsd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestReqMethodValidate(t *testing.T) {
	req := &http.Request{}
	w := NewMockResponseWriter()
	req.Method = "GET"
	if !reqMethodValidate(w, req, "GET") {
		t.Error("validation failed")
	}
	if len(w.ResBytes) > 0 {
		t.Error("response inserted")
	}
	req.Method = "POST"
	if reqMethodValidate(w, req, "GET") {
		t.Error("validation failed")
	}
	if w.ResponseString() != "Method Not Allowed" {
		t.Error("response body failed")
	}
	if w.StatusCode != http.StatusMethodNotAllowed {
		t.Error("response code failed")
	}
}

func TestRenderJSON(t *testing.T) {
	w := NewMockResponseWriter()
	renderJSON(w, &StatSuccessResponse{
		Success: true,
	})
	if w.ResponseString() != `{"success":true}` {
		t.Errorf("response body failed: %s\n", w.ResponseString())
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("response type failed")
	}
	w.ResBytes = []byte{} // clear
	renderJSON(w, &StatCurrentJobsResponse{
		CurrentJobs: []QueueSummary{
			QueueSummary{ID: "1", Payload: "p1", ReceivedAt: 10},
		},
	})
	var r StatCurrentJobsResponse
	if err := json.Unmarshal(w.ResBytes, &r); err != nil {
		t.Error("json unmarshal error", err)
	}
	if len(r.CurrentJobs) != 1 {
		t.Error("current_jobs count invalid")
	}
	if r.CurrentJobs[0].ID != "1" {
		t.Error("job id invalid")
	}
}

func TestWorkerStatsAndJobsHandler(t *testing.T) {
	tr := NewQueueTracker(5, NewLogger("DEBUG"))
	h := &StatHandler{tr}

	for i := 1; i <= 5; i++ {
		tr.Register(Queue{
			ID:      fmt.Sprintf("id:%d", i),
			Payload: "foobar",
			Receipt: fmt.Sprintf("reciept:%d", i),
		})
	}

	workerStatsController := h.WorkerStatsHandler()
	req := &http.Request{}
	req.Method = "POST"

	t.Run("invalid Method for summary", func(t *testing.T) {
		w := NewMockResponseWriter()
		workerStatsController(w, req)

		if w.StatusCode != http.StatusMethodNotAllowed {
			t.Error("response error found")
		}
	})

	t.Run("valid Method for summary", func(t *testing.T) {
		w := NewMockResponseWriter()
		req.Method = "GET"
		workerStatsController(w, req)

		if w.StatusCode != http.StatusOK {
			t.Error("response error found")
		}

		var r StatWorkerStatsResponse
		if err := json.Unmarshal(w.ResBytes, &r); err != nil {
			t.Error("json unmarshal error", err)
		}

		if len(tr.CurrentSummaries()) != 5 {
			t.Error("job summaries invalid")
		}

		if !r.IsWorking {
			t.Error("is_working invalid")
		}
	})

	jobsController := h.WorkerCurrentJobsHandler()
	req.Method = "POST"

	t.Run("invalid Method for jobs", func(t *testing.T) {
		w := NewMockResponseWriter()
		jobsController(w, req)

		if w.StatusCode == http.StatusOK {
			t.Error("response error found")
		}
	})

	t.Run("valid Method for jobs", func(t *testing.T) {
		w := NewMockResponseWriter()
		req.Method = "GET"
		jobsController(w, req)

		if w.StatusCode != http.StatusOK {
			t.Error("response error found")
		}

		var r StatCurrentJobsResponse
		if err := json.Unmarshal(w.ResBytes, &r); err != nil {
			t.Error("json unmarshal error", err)
		}

		if len(r.CurrentJobs) != 5 {
			t.Error("current_jobs count invalid")
		}

		for _, summary := range r.CurrentJobs {
			if _, exists := tr.CurrentWorkings.Load(summary.ID); !exists {
				t.Errorf("job summary not registered: %s\n", summary.ID)
			}
		}
	})
}

func TestWorkerPauseAndResumeHandler(t *testing.T) {
	tr := NewQueueTracker(5, NewLogger("DEBUG"))
	h := &StatHandler{tr}

	pauseController := h.WorkerPauseHandler()

	req := &http.Request{}

	req.Method = "GET"
	t.Run("pause failed", func(t *testing.T) {
		w := NewMockResponseWriter()
		pauseController(w, req)

		if w.StatusCode != http.StatusMethodNotAllowed {
			t.Error("response code invalid")
		}

		if !tr.IsWorking() {
			t.Error("IsWorking changed")
		}
	})

	req.Method = "POST"
	t.Run("pause success", func(t *testing.T) {
		w := NewMockResponseWriter()
		pauseController(w, req)

		if w.StatusCode != http.StatusOK {
			t.Error("response code invalid")
		}

		if tr.IsWorking() {
			t.Error("IsWorking not changed")
		}
	})

	resumeController := h.WorkerResumeHandler()

	req.Method = "GET"
	t.Run("resume failed", func(t *testing.T) {
		w := NewMockResponseWriter()
		resumeController(w, req)

		if w.StatusCode != http.StatusMethodNotAllowed {
			t.Error("response code invalid")
		}

		if tr.IsWorking() {
			t.Error("IsWorking changed")
		}
	})

	req.Method = "POST"
	t.Run("resume success", func(t *testing.T) {
		w := NewMockResponseWriter()
		resumeController(w, req)

		if w.StatusCode != http.StatusOK {
			t.Error("response code invalid")
		}

		if !tr.IsWorking() {
			t.Error("IsWorking not changed")
		}
	})

}

func TestStatHandlerServeMux(t *testing.T) {
	tr := NewQueueTracker(5, NewLogger("DEBUG"))
	h := &StatHandler{tr}

	if m := h.BuildServeMux(); m == nil {
		t.Error("ServeMux not returned.")
	}
}
