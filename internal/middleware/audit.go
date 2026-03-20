package middleware

import (
	"database/sql"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/config"
	"github.com/hkdb/lmgate/internal/metrics"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

var auditRequestCounter int64

const auditBatchSize = 100

var OnFlush func()

var auditChan chan models.AuditLog
var resetChan chan struct{}
var flushIntervalChan chan time.Duration

func StartAuditWorker(db *sql.DB, flushInterval time.Duration) {
	auditChan = make(chan models.AuditLog, 10000)
	resetChan = make(chan struct{}, 1)
	flushIntervalChan = make(chan time.Duration, 1)

	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}

	go func() {
		batch := make([]models.AuditLog, 0, auditBatchSize)
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()

		for {
			select {
			case entry := <-auditChan:
				batch = append(batch, entry)
				if len(batch) < auditBatchSize {
					continue
				}
			case <-ticker.C:
				if len(batch) == 0 {
					continue
				}
			case newInterval := <-flushIntervalChan:
				ticker.Reset(newInterval)
				continue
			case <-resetChan:
				batch = batch[:0]
				for {
					select {
					case <-auditChan:
					default:
						goto drained
					}
				}
			drained:
				continue
			}

			if err := models.InsertAuditLogs(db, batch); err != nil {
				log.Printf("audit flush error: %v", err)
			}
			if OnFlush != nil {
				OnFlush()
			}
			batch = batch[:0]
		}
	}()
}

func SetAuditFlushInterval(d time.Duration) {
	if flushIntervalChan != nil && d > 0 {
		select {
		case flushIntervalChan <- d:
		default:
		}
	}
}

func ResetAuditBuffer() {
	if resetChan != nil {
		select {
		case resetChan <- struct{}{}:
		default:
		}
	}
}

func Audit(collector *metrics.Collector, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		reqID := fmt.Sprintf("audit-%d-%d", start.UnixNano(), atomic.AddInt64(&auditRequestCounter, 1))
		c.Locals("request_id", reqID)
		c.Locals("request_start", start)

		err := c.Next()

		totalDuration := time.Since(start)
		latency := totalDuration.Milliseconds()
		u := auth.GetUser(c)

		statusCode := c.Response().StatusCode()
		if proxyStatus, ok := c.Locals("proxy_status").(int); ok {
			statusCode = proxyStatus
		}

		var upstreamTTFB int64
		if v, ok := c.Locals("upstream_ttfb_ms").(int64); ok {
			upstreamTTFB = v
		}
		gatewayOverheadUs := totalDuration.Microseconds() - (upstreamTTFB * 1000)
		if gatewayOverheadUs < 0 {
			gatewayOverheadUs = 0
		}

		var streaming bool
		if v, ok := c.Locals("response_streaming").(bool); ok {
			streaming = v
		}

		var modelName *string
		if m, ok := c.Locals("request_model").(string); ok && m != "" {
			modelName = &m
		}

		logType := "admin"
		if modelName != nil {
			logType = "api"
		}

		entry := models.AuditLog{
			Method:            c.Method(),
			Path:              c.Path(),
			Model:             modelName,
			StatusCode:        statusCode,
			LatencyMs:         latency,
			GatewayOverheadUs: gatewayOverheadUs,
			UpstreamTTFBMs:    upstreamTTFB,
			Streaming:         streaming,
			RequestID:         reqID,
			LogType:           logType,
		}

		ip := c.IP()
		entry.IPAddr = &ip
		if v := c.Get("X-Real-IP"); v != "" {
			entry.XRealIP = &v
		}
		if v := c.Get("X-Forwarded-For"); v != "" {
			entry.XForwardedFor = &v
		}

		if u != nil {
			entry.UserID = &u.UserID
			if u.TokenID != "" {
				entry.TokenID = &u.TokenID
			}
		}

		go func() {
			if u != nil && modelName != nil {
				collector.Record(u.UserID, *modelName, 0, 0)
			}

			// Gate log writes on per-type enabled flags
			switch entry.LogType {
			case "api":
				if !cfg.Logging.APILogEnabled {
					return
				}
			case "admin":
				if !cfg.Logging.AdminLogEnabled {
					return
				}
			}

			if auditChan != nil {
				select {
				case auditChan <- entry:
				default:
					log.Printf("audit channel full, dropping entry")
				}
			}
		}()

		return err
	}
}
