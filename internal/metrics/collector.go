package metrics

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/hkdb/lmgate/internal/models"
)

type metricKey struct {
	UserID string
	Model  string
	Period string
}

type metricValue struct {
	RequestCount     int64
	PromptTokens     int64
	CompletionTokens int64
}

type Collector struct {
	db       *sql.DB
	mu       sync.Mutex
	counters map[metricKey]*metricValue
	done     chan struct{}
}

func NewCollector(db *sql.DB, flushInterval time.Duration) *Collector {
	c := &Collector{
		db:       db,
		counters: make(map[metricKey]*metricValue),
		done:     make(chan struct{}),
	}

	go c.flushLoop(flushInterval)
	return c
}

func (c *Collector) Record(userID, model string, promptTokens, completionTokens int64) {
	if model == "" {
		model = "unknown"
	}

	now := time.Now().UTC()
	period := now.Truncate(time.Hour).Format(time.DateTime)

	key := metricKey{UserID: userID, Model: model, Period: period}

	c.mu.Lock()
	v, ok := c.counters[key]
	if !ok {
		v = &metricValue{}
		c.counters[key] = v
	}
	v.RequestCount++
	v.PromptTokens += promptTokens
	v.CompletionTokens += completionTokens
	c.mu.Unlock()
}

// RecordTokens records token usage without incrementing the request count.
// Used by the proxy handler goroutines that extract tokens after the audit
// middleware has already counted the request.
func (c *Collector) RecordTokens(userID, model string, promptTokens, completionTokens int64) {
	if promptTokens == 0 && completionTokens == 0 {
		return
	}
	if model == "" {
		model = "unknown"
	}

	now := time.Now().UTC()
	period := now.Truncate(time.Hour).Format(time.DateTime)

	key := metricKey{UserID: userID, Model: model, Period: period}

	c.mu.Lock()
	v, ok := c.counters[key]
	if !ok {
		v = &metricValue{}
		c.counters[key] = v
	}
	v.PromptTokens += promptTokens
	v.CompletionTokens += completionTokens
	c.mu.Unlock()
}

func (c *Collector) flushLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Flush()
		case <-c.done:
			c.Flush()
			return
		}
	}
}

// OnFlush is called after each metrics flush cycle completes.
var OnFlush func()

func (c *Collector) Flush() {
	c.mu.Lock()
	snapshot := c.counters
	c.counters = make(map[metricKey]*metricValue)
	c.mu.Unlock()

	for key, val := range snapshot {
		periodStart, _ := time.Parse(time.DateTime, key.Period)
		periodEnd := periodStart.Add(time.Hour)

		m := models.UsageMetric{
			UserID:           key.UserID,
			Model:            key.Model,
			PromptTokens:     val.PromptTokens,
			CompletionTokens: val.CompletionTokens,
			RequestCount:     val.RequestCount,
			PeriodStart:      key.Period,
			PeriodEnd:        periodEnd.Format(time.DateTime),
		}

		if err := models.UpsertUsageMetric(c.db, m); err != nil {
			log.Printf("metrics flush error: %v", err)
		}
	}

	if OnFlush != nil {
		OnFlush()
	}
}

func (c *Collector) Reset() {
	c.mu.Lock()
	c.counters = make(map[metricKey]*metricValue)
	c.mu.Unlock()
}

func (c *Collector) Stop() {
	close(c.done)
}
