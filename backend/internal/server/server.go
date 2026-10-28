// Package server bounds HTTP resources and owns graceful process lifecycle.
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/response"
)

const MaxBodyBytes int64 = 8 << 20

// RequestBounds is applied after request-ID middleware. The streaming reader
// also bounds chunked requests; binding errors remain endpoint-specific.
func RequestBounds(maxActive int) gin.HandlerFunc {
	active := make(chan struct{}, maxActive)
	readiness := make(chan struct{}, 4)
	return func(c *gin.Context) {
		if c.Request.ContentLength > MaxBodyBytes {
			response.Error(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "请求内容超过 8 MiB 限制", nil)
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxBodyBytes)
		// Liveness does not touch the DB. Readiness gets a separate small budget
		// so it cannot consume every database connection during overload.
		if c.Request.URL.Path == "/health/live" {
			c.Next()
			return
		}
		budget := active
		if c.Request.URL.Path == "/health/ready" {
			budget = readiness
		}
		select {
		case budget <- struct{}{}:
			defer func() { <-budget }()
		default:
			c.Header("Retry-After", "1")
			response.Error(c, http.StatusServiceUnavailable, "SERVER_BUSY", "服务繁忙，请稍后重试", nil)
			c.Abort()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

// Serve drains accepted requests on shutdown, then forcibly closes only if the
// drain budget expires. Background workers use the same cancellation signal.
func Serve(ctx context.Context, srv *http.Server, listener net.Listener) error {
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- srv.Serve(listener) }()
	select {
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return err
		}
		if err := <-errorsCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// Periodic never overlaps a job with itself and does not start a new iteration
// after cancellation. The caller must wait for workers before closing its DB.
func Periodic(ctx context.Context, workers *sync.WaitGroup, interval, budget time.Duration, job func(context.Context)) {
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			jobCtx, cancel := context.WithTimeout(ctx, budget)
			job(jobCtx)
			cancel()
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}
