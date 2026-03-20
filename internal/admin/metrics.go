package admin

import (
	"bufio"
	"database/sql"
	"time"

	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
)

type usageMetricsResponse struct {
	DailyRequests                        []dailyRequest `json:"daily_requests"`
	ModelUsage                           []modelCount   `json:"model_usage"`
	UserUsage                            []userCount    `json:"user_usage"`
	TokenUsage                           []tokenCount   `json:"token_usage"`
	DailyTimings                         []dailyTiming  `json:"daily_timings"`
	AvgGatewayOverheadMs                 float64        `json:"avg_gateway_overhead_ms"`
	AvgUpstreamTTFBMs                    float64        `json:"avg_upstream_ttfb_ms"`
	AvgStreamingGatewayOverheadMs        float64        `json:"avg_streaming_gateway_overhead_ms"`
	AvgUpstreamStreamingTTFBMs           float64        `json:"avg_upstream_streaming_ttfb_ms"`
	AvgResponseTimeMs                    float64        `json:"avg_response_time_ms"`
	AvgResponseTimeNonStreamingMs        float64        `json:"avg_response_time_non_streaming_ms"`
	AvgResponseTimeStreamingMs           float64        `json:"avg_response_time_streaming_ms"`
	TotalTokensUsed                      int64          `json:"total_tokens_used"`
	TotalRequests                        int64          `json:"total_requests"`
	StreamingRequests                    int64          `json:"streaming_requests"`
	NonStreamingRequests                 int64          `json:"non_streaming_requests"`
	ErrorRequests                        int64          `json:"error_requests"`
	StreamingErrorRequests               int64          `json:"streaming_error_requests"`
	NonStreamingErrorRequests            int64          `json:"non_streaming_error_requests"`
}

type dailyRequest struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type modelCount struct {
	Model string `json:"model"`
	Count int64  `json:"count"`
}

type userCount struct {
	Email string `json:"email"`
	Count int64  `json:"count"`
}

type tokenCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type dailyTiming struct {
	Date                 string  `json:"date"`
	Streaming            int     `json:"streaming"`
	AvgGatewayOverheadMs float64 `json:"avg_gateway_overhead_ms"`
	AvgUpstreamTTFBMs    float64 `json:"avg_upstream_ttfb_ms"`
	AvgResponseTimeMs    float64 `json:"avg_response_time_ms"`
}

func (a *Admin) StreamMetrics(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	id, ch := a.MetricsNotifier.Subscribe()

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer a.MetricsNotifier.Unsubscribe(id)

		// Flush an SSE comment to push headers through TLS buffering
		if _, err := w.WriteString(": connected\n\n"); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return
				}
				if _, err := w.WriteString("data: refresh\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case <-heartbeat.C:
				if _, err := w.WriteString(": heartbeat\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})

	return nil
}

func (a *Admin) GetUsageMetrics(c *fiber.Ctx) error {
	period := c.Query("period", "7d")

	var duration time.Duration
	switch period {
	case "24h":
		duration = 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	case "90d":
		duration = 90 * 24 * time.Hour
	default:
		duration = 7 * 24 * time.Hour
	}

	since := time.Now().UTC().Add(-duration).Format(time.DateTime)
	resp := usageMetricsResponse{}

	// Total tokens from usage_metrics
	err := a.DB.QueryRow(
		`SELECT COALESCE(SUM(prompt_tokens + completion_tokens), 0)
		 FROM usage_metrics WHERE period_start >= ?`, since,
	).Scan(&resp.TotalTokensUsed)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get metrics"})
	}

	// Split metrics by streaming vs non-streaming
	streamRows, err := a.DB.Query(
		`SELECT streaming, COUNT(*), AVG(gateway_overhead_us) / 1000.0, AVG(upstream_ttfb_ms), AVG(latency_ms), AVG(latency_ms - upstream_ttfb_ms)
		 FROM audit_logs WHERE created_at >= ? AND log_type = 'api' GROUP BY streaming`, since,
	)
	if err == nil {
		defer streamRows.Close()
		var totalLatencyWeighted float64
		var totalStreamCount int64
		for streamRows.Next() {
			var streaming int
			var count int64
			var avgOverhead, avgTTFB, avgLatency, avgRespTime sql.NullFloat64
			if err := streamRows.Scan(&streaming, &count, &avgOverhead, &avgTTFB, &avgLatency, &avgRespTime); err != nil {
				continue
			}
			switch streaming {
			case 0:
				resp.NonStreamingRequests = count
				if avgOverhead.Valid {
					resp.AvgGatewayOverheadMs = avgOverhead.Float64
				}
				if avgTTFB.Valid {
					resp.AvgUpstreamTTFBMs = avgTTFB.Float64
				}
				if avgRespTime.Valid {
					resp.AvgResponseTimeNonStreamingMs = avgRespTime.Float64
				}
			case 1:
				resp.StreamingRequests = count
				if avgOverhead.Valid {
					resp.AvgStreamingGatewayOverheadMs = avgOverhead.Float64
				}
				if avgTTFB.Valid {
					resp.AvgUpstreamStreamingTTFBMs = avgTTFB.Float64
				}
				if avgRespTime.Valid {
					resp.AvgResponseTimeStreamingMs = avgRespTime.Float64
				}
			}
			if avgLatency.Valid {
				totalLatencyWeighted += avgLatency.Float64 * float64(count)
				totalStreamCount += count
			}
		}
		if totalStreamCount > 0 {
			resp.AvgResponseTimeMs = totalLatencyWeighted / float64(totalStreamCount)
		}
		resp.TotalRequests = resp.StreamingRequests + resp.NonStreamingRequests
	}

	// Error counts by streaming type
	errRows, err := a.DB.Query(
		`SELECT streaming, COUNT(*) FROM audit_logs
		 WHERE created_at >= ? AND status_code >= 400 AND log_type = 'api'
		 GROUP BY streaming`, since,
	)
	if err == nil {
		defer errRows.Close()
		for errRows.Next() {
			var streaming int
			var count int64
			if err := errRows.Scan(&streaming, &count); err != nil {
				continue
			}
			switch streaming {
			case 0:
				resp.NonStreamingErrorRequests = count
			case 1:
				resp.StreamingErrorRequests = count
			}
		}
		resp.ErrorRequests = resp.StreamingErrorRequests + resp.NonStreamingErrorRequests
	}

	// Daily requests
	rows, err := a.DB.Query(
		`SELECT DATE(period_start) as d, SUM(request_count)
		 FROM usage_metrics WHERE period_start >= ?
		 GROUP BY d ORDER BY d`, since,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get daily requests"})
	}
	defer rows.Close()
	for rows.Next() {
		var dr dailyRequest
		if err := rows.Scan(&dr.Date, &dr.Count); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan daily requests"})
		}
		resp.DailyRequests = append(resp.DailyRequests, dr)
	}
	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to iterate daily requests"})
	}

	// Model usage
	modelRows, err := a.DB.Query(
		`SELECT model, SUM(request_count)
		 FROM usage_metrics WHERE period_start >= ?
		 GROUP BY model ORDER BY SUM(request_count) DESC`, since,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get model usage"})
	}
	defer modelRows.Close()
	for modelRows.Next() {
		var mc modelCount
		if err := modelRows.Scan(&mc.Model, &mc.Count); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan model usage"})
		}
		resp.ModelUsage = append(resp.ModelUsage, mc)
	}
	if err := modelRows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to iterate model usage"})
	}

	// User usage
	userRows, err := a.DB.Query(
		`SELECT u.email, SUM(um.request_count)
		 FROM usage_metrics um JOIN users u ON um.user_id = u.id
		 WHERE um.period_start >= ?
		 GROUP BY um.user_id ORDER BY SUM(um.request_count) DESC`, since,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get user usage"})
	}
	defer userRows.Close()
	for userRows.Next() {
		var uc userCount
		if err := userRows.Scan(&uc.Email, &uc.Count); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan user usage"})
		}
		resp.UserUsage = append(resp.UserUsage, uc)
	}
	if err := userRows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to iterate user usage"})
	}

	// Token usage
	tokenRows, err := a.DB.Query(
		`SELECT COALESCE(t.name, 'unknown'), COUNT(*)
		 FROM audit_logs a LEFT JOIN api_tokens t ON a.token_id = t.id
		 WHERE a.created_at >= ? AND a.token_id IS NOT NULL AND a.log_type = 'api'
		 GROUP BY a.token_id ORDER BY COUNT(*) DESC`, since,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get token usage"})
	}
	defer tokenRows.Close()
	for tokenRows.Next() {
		var tc tokenCount
		if err := tokenRows.Scan(&tc.Name, &tc.Count); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan token usage"})
		}
		resp.TokenUsage = append(resp.TokenUsage, tc)
	}
	if err := tokenRows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to iterate token usage"})
	}

	// Daily timing breakdown
	timingRows, err := a.DB.Query(
		`SELECT DATE(created_at), streaming,
		        AVG(gateway_overhead_us) / 1000.0,
		        AVG(upstream_ttfb_ms),
		        AVG(latency_ms - upstream_ttfb_ms)
		 FROM audit_logs
		 WHERE created_at >= ? AND log_type = 'api'
		 GROUP BY DATE(created_at), streaming
		 ORDER BY DATE(created_at)`, since,
	)
	if err == nil {
		defer timingRows.Close()
		for timingRows.Next() {
			var dt dailyTiming
			var avgOverhead, avgTTFB, avgRespTime sql.NullFloat64
			if err := timingRows.Scan(&dt.Date, &dt.Streaming, &avgOverhead, &avgTTFB, &avgRespTime); err != nil {
				continue
			}
			if avgOverhead.Valid {
				dt.AvgGatewayOverheadMs = avgOverhead.Float64
			}
			if avgTTFB.Valid {
				dt.AvgUpstreamTTFBMs = avgTTFB.Float64
			}
			if avgRespTime.Valid {
				dt.AvgResponseTimeMs = avgRespTime.Float64
			}
			resp.DailyTimings = append(resp.DailyTimings, dt)
		}
	}

	// Ensure slices are non-nil for JSON encoding
	if resp.DailyRequests == nil {
		resp.DailyRequests = []dailyRequest{}
	}
	if resp.ModelUsage == nil {
		resp.ModelUsage = []modelCount{}
	}
	if resp.UserUsage == nil {
		resp.UserUsage = []userCount{}
	}
	if resp.TokenUsage == nil {
		resp.TokenUsage = []tokenCount{}
	}
	if resp.DailyTimings == nil {
		resp.DailyTimings = []dailyTiming{}
	}

	return c.JSON(resp)
}

func (a *Admin) GetMetricsSummary(c *fiber.Ctx) error {
	since := c.Query("since", time.Now().UTC().Add(-24*time.Hour).Format(time.DateTime))

	summary, err := models.GetUsageSummary(a.DB, since)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get metrics"})
	}

	return c.JSON(summary)
}

func (a *Admin) GetUserMetrics(c *fiber.Ctx) error {
	userID := c.Params("id")
	since := c.Query("since", time.Now().UTC().Add(-24*time.Hour).Format(time.DateTime))

	metrics, err := models.GetUserUsage(a.DB, userID, since)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get user metrics"})
	}

	return c.JSON(fiber.Map{"metrics": metrics})
}
