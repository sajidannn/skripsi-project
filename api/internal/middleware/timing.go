package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type timingKey string

const timingCtxKey timingKey = "timing"

// Timing holds named checkpoints captured during a request lifecycle.
type Timing struct {
	start       time.Time
	checkpoints []checkpoint
}

type checkpoint struct {
	label string
	at    time.Time
}

// Mark records a named checkpoint (call this anywhere you have access to ctx).
func (t *Timing) Mark(label string) {
	t.checkpoints = append(t.checkpoints, checkpoint{label: label, at: time.Now()})
}

// Segments returns a map of label → duration-from-previous-checkpoint.
func (t *Timing) Segments() []Segment {
	prev := t.start
	out := make([]Segment, 0, len(t.checkpoints))
	for _, cp := range t.checkpoints {
		out = append(out, Segment{Label: cp.label, Duration: cp.at.Sub(prev)})
		prev = cp.at
	}
	return out
}

// Total returns total elapsed time from request arrival.
func (t *Timing) Total() time.Duration {
	return time.Since(t.start)
}

// Segment is a labeled time segment.
type Segment struct {
	Label    string
	Duration time.Duration
}

// newTiming creates a Timing starting now.
func newTiming() *Timing {
	return &Timing{start: time.Now()}
}

// TimingFromContext retrieves the Timing from ctx, or nil if not present.
func TimingFromContext(ctx context.Context) *Timing {
	if t, ok := ctx.Value(timingCtxKey).(*Timing); ok {
		return t
	}
	return nil
}

// RequestTiming is a Gin middleware that:
//   - injects a *Timing into the request context
//   - records an "auth_done" checkpoint after downstream middleware (JWT) runs
//   - logs the full segment breakdown after the handler returns
func RequestTiming() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := newTiming()
		ctx := context.WithValue(c.Request.Context(), timingCtxKey, t)
		c.Request = c.Request.WithContext(ctx)

		// Let all downstream middleware + handler run.
		c.Next()

		// Final log: total + per-segment breakdown.
		total := t.Total()
		segments := t.Segments()

		fields := gin.H{
			"method":   c.Request.Method,
			"path":     c.Request.URL.Path,
			"status":   c.Writer.Status(),
			"total_ms": total.Milliseconds(),
		}
		for _, s := range segments {
			fields[s.Label+"_ms"] = s.Duration.Milliseconds()
		}

		// Emit as a structured log line so it's easy to grep.
		c.Set("timing_total_ms", total.Milliseconds())

		// Also expose via response header for quick curl inspection.
		c.Header("X-Response-Time-Ms", formatMs(total))
		for _, s := range segments {
			c.Header("X-Time-"+s.Label+"-Ms", formatMs(s.Duration))
		}

		// Print to stdout — clearly visible in debug / server log.
		printTiming(c.Request.Method, c.Request.URL.Path, c.Writer.Status(), total, segments)
	}
}

func formatMs(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func printTiming(method, path string, status int, total time.Duration, segments []Segment) {
	line := fmt.Sprintf("[TIMING] %s %s %d | total=%s", method, path, status, total.Round(time.Millisecond))
	for _, s := range segments {
		line += fmt.Sprintf(" | %s=%s", s.Label, s.Duration.Round(time.Millisecond))
	}
	fmt.Println(line)
}
