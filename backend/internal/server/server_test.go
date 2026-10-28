package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestOversizedKnownAndChunkedBodiesAreBounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestBounds(1))
	r.POST("/body", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(413)
			return
		}
		c.Status(204)
	})
	for _, length := range []int64{MaxBodyBytes + 1, -1} {
		req := httptest.NewRequest("POST", "/body", strings.NewReader(strings.Repeat("x", int(MaxBodyBytes+1))))
		req.ContentLength = length
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		if response.Code != 413 {
			t.Fatalf("length %d returned %d", length, response.Code)
		}
	}
}

func TestOverloadRejectsNormalRequestsButKeepsLiveness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestBounds(1))
	entered, release := make(chan struct{}), make(chan struct{})
	r.GET("/busy", func(c *gin.Context) { close(entered); <-release; c.Status(204) })
	r.GET("/other", func(c *gin.Context) { c.Status(204) })
	r.GET("/health/live", func(c *gin.Context) { c.Status(200) })
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/busy", nil))
	}()
	<-entered
	for path, want := range map[string]int{"/other": 503, "/health/live": 200} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != want {
			t.Errorf("%s = %d, want %d", path, response.Code, want)
		}
	}
	close(release)
	<-finished
}

func TestGracefulShutdownWaitsForAcceptedRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	srv := New(listener.Addr().String(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "committed")
	}))
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, srv, listener) }()
	responseDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			defer response.Body.Close()
			var body []byte
			body, err = io.ReadAll(response.Body)
			if string(body) != "committed" {
				t.Errorf("in-flight response was truncated: %q", body)
			}
		}
		responseDone <- err
	}()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("shutdown did not wait for accepted request")
	default:
	}
	close(release)
	if err := <-responseDone; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPeriodicCancelsRunningJobAndDoesNotRestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	var calls atomic.Int32
	started := make(chan struct{})
	Periodic(ctx, &workers, time.Millisecond, time.Minute, func(jobCtx context.Context) {
		calls.Add(1)
		close(started)
		<-jobCtx.Done()
	})
	<-started
	cancel()
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker ignored cancellation")
	}
	if calls.Load() != 1 {
		t.Fatalf("worker ran %d times", calls.Load())
	}
}

func TestSlowHeadersCannotHoldConnectionIndefinitely(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(listener.Addr().String(), http.NotFoundHandler())
	srv.ReadHeaderTimeout = 40 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, srv, listener) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost:")
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 256)
	_, err = conn.Read(buffer)
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("slow request kept connection open beyond header timeout")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReadinessHasItsOwnBoundedBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestBounds(1))
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	r.GET("/health/ready", func(c *gin.Context) { entered <- struct{}{}; <-release; c.Status(200) })
	r.GET("/normal", func(c *gin.Context) { c.Status(204) })
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/health/ready", nil))
		}()
	}
	for i := 0; i < 4; i++ {
		<-entered
	}
	for path, want := range map[string]int{"/health/ready": 503, "/normal": 204} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Errorf("%s got %d, want %d", path, w.Code, want)
		}
	}
	close(release)
	workers.Wait()
}
