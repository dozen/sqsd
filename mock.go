package sqsd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

// MockClient provides mocking sqs library from aws-sdk-go for test
type MockClient struct {
	sqsiface.SQSAPI
	Resp             *sqs.ReceiveMessageOutput
	RecvRequestCount int
	DelRequestCount  int
	ErrRequestCount  int
	Err              error
	mu               sync.Mutex
	RecvFunc         func(*sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error)
}

// NewMockClient returns MockClient object
func NewMockClient() *MockClient {
	c := &MockClient{
		Resp: &sqs.ReceiveMessageOutput{
			Messages: []*sqs.Message{},
		},
		mu: sync.Mutex{},
	}
	c.RecvFunc = func(param *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
		c.mu.Lock()
		c.RecvRequestCount++
		c.mu.Unlock()
		if len(c.Resp.Messages) == 0 && *param.WaitTimeSeconds > 0 {
			dur := time.Duration(*param.WaitTimeSeconds)
			time.Sleep(dur * time.Second)
		}
		if c.Err != nil {
			c.ErrRequestCount++
		}
		return c.Resp, c.Err
	}
	return c
}

// ReceiveMessageWithContext is mock for same name method
func (c *MockClient) ReceiveMessageWithContext(ctx aws.Context, param *sqs.ReceiveMessageInput, opts ...request.Option) (*sqs.ReceiveMessageOutput, error) {
	o, e := c.RecvFunc(param)
	return o, e
}

// DeleteMessage is mock for same name method
func (c *MockClient) DeleteMessage(*sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	c.mu.Lock()
	c.DelRequestCount++
	c.mu.Unlock()
	return &sqs.DeleteMessageOutput{}, nil
}

// MockServer provides test server with several response like error, long-time, and ok
func MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-Type", "text")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "no goood")
	})
	mux.HandleFunc("/long", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-Type", "text")
		fmt.Fprintf(w, "goood")
		time.Sleep(1 * time.Second)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-Type", "text")
		fmt.Fprintf(w, "goood")
	})
	return httptest.NewServer(mux)
}

// MockResponseWriter provides stat handler's response writer
type MockResponseWriter struct {
	http.ResponseWriter
	header     http.Header
	ResBytes   []byte
	StatusCode int
	Err        error
}

// NewMockResponseWriter returns MockResponseWriter object
func NewMockResponseWriter() *MockResponseWriter {
	return &MockResponseWriter{
		header:     http.Header{},
		ResBytes:   []byte{},
		StatusCode: http.StatusOK,
	}
}

// Header returns http.Header object
func (w *MockResponseWriter) Header() http.Header {
	return w.header
}

// Write provides setting arguments to property itself
func (w *MockResponseWriter) Write(b []byte) (int, error) {
	w.ResBytes = b
	return len(b), w.Err
}

// WriteHeader provides setting arguments to property itself
func (w *MockResponseWriter) WriteHeader(s int) {
	w.StatusCode = s
}

// ResponseString returns string given from Write method
func (w *MockResponseWriter) ResponseString() string {
	return bytes.NewBuffer(w.ResBytes).String()
}
