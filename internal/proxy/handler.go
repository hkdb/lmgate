package proxy

import (
	"bufio"
	"bytes"
	"database/sql"
	"io"
	"log"
	"time"

	"github.com/hkdb/lmgate/internal/auth"
	"github.com/hkdb/lmgate/internal/metrics"
	"github.com/hkdb/lmgate/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	client      *fasthttp.Client
	upstreamURL string
	collector   *metrics.Collector
	db          *sql.DB
}

func New(upstreamURL string, timeout time.Duration, collector *metrics.Collector, responseBodyLimit int, db *sql.DB) *Handler {
	return &Handler{
		upstreamURL: upstreamURL,
		collector:   collector,
		db:          db,
		client: &fasthttp.Client{
			ReadTimeout:         timeout,
			WriteTimeout:        30 * time.Second,
			MaxResponseBodySize: responseBodyLimit,
			StreamResponseBody:  true,
		},
	}
}

func (h *Handler) Handle(c *fiber.Ctx) error {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	// NO defer release — we manage lifecycle manually to avoid
	// releasing the response while the stream writer callback is still reading

	// Build upstream request
	upstreamURI := h.upstreamURL + string(c.Request().URI().RequestURI())
	req.SetRequestURI(upstreamURI)
	req.Header.SetMethod(c.Method())

	// Copy headers (skip hop-by-hop)
	c.Request().Header.VisitAll(func(key, value []byte) {
		k := string(key)
		switch k {
		case "Connection", "Keep-Alive", "Transfer-Encoding", "TE", "Trailer",
			"Upgrade", "Proxy-Authorization", "Proxy-Connection", "Host":
			return
		}
		req.Header.SetBytesKV(key, value)
	})

	// Remove Authorization header from upstream request
	req.Header.Del("Authorization")

	// Copy body
	if len(c.Body()) > 0 {
		req.SetBody(c.Body())
	}

	resp.StreamBody = true

	upstreamStart := time.Now()
	if err := h.client.Do(req, resp); err != nil {
		log.Printf("proxy error: %v", err)
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "upstream unavailable"})
	}
	c.Locals("upstream_ttfb_ms", time.Since(upstreamStart).Milliseconds())

	// Copy response headers
	resp.Header.VisitAll(func(key, value []byte) {
		k := string(key)
		switch k {
		case "Connection", "Keep-Alive", "Transfer-Encoding":
			return
		}
		c.Response().Header.SetBytesKV(key, value)
	})

	statusCode := resp.StatusCode()
	c.Status(statusCode)
	c.Locals("proxy_status", statusCode)

	// Capture user context for token recording goroutines
	userID, model := h.getUserAndModel(c)
	var requestID string
	if rid, ok := c.Locals("request_id").(string); ok {
		requestID = rid
	}
	var requestStart time.Time
	if rs, ok := c.Locals("request_start").(time.Time); ok {
		requestStart = rs
	}
	bodyStream := resp.BodyStream()
	if bodyStream == nil {
		c.Locals("response_streaming", false)
		body := resp.Body()
		bodyCopy := make([]byte, len(body))
		copy(bodyCopy, body)
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)
		if err := c.Send(bodyCopy); err != nil {
			return err
		}
		go h.recordNonStreamingTokens(userID, model, bodyCopy, requestID)
		return nil
	}

	// Detect streaming from response content-type or transfer encoding
	respCT := string(resp.Header.ContentType())
	isStreaming := respCT == "text/event-stream" ||
		respCT == "application/x-ndjson"

	c.Locals("response_streaming", isStreaming)

	if !isStreaming {
		body, err := io.ReadAll(bodyStream)
		if err != nil {
			body = resp.Body()
		}
		bodyCopy := make([]byte, len(body))
		copy(bodyCopy, body)
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)
		if err := c.Send(bodyCopy); err != nil {
			return err
		}
		go h.recordNonStreamingTokens(userID, model, bodyCopy, requestID)
		return nil
	}

	// Streaming: release request now, release response after stream completes
	fasthttp.ReleaseRequest(req)

	c.Response().Header.Set("Cache-Control", "no-cache")
	c.Response().Header.Set("Connection", "keep-alive")
	c.Response().Header.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer fasthttp.ReleaseResponse(resp)

		var lastUsageLine []byte
		var partial []byte
		usageMarkers := [][]byte{
			[]byte("\"usage\""),            // OpenAI, LM Studio, llama.cpp /v1/
			[]byte("\"eval_count\""),       // Ollama native
			[]byte("\"tokens_predicted\""), // llama.cpp / BitNet native /completion
		}
		buf := make([]byte, 4096)

		for {
			n, err := bodyStream.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if _, writeErr := w.Write(chunk); writeErr != nil {
					return
				}
				if flushErr := w.Flush(); flushErr != nil {
					return
				}

				// Scan for usage in SSE / NDJSON lines
				partial = append(partial, chunk...)
				for {
					idx := bytes.IndexByte(partial, '\n')
					if idx < 0 {
						break
					}
					line := partial[:idx]
					partial = partial[idx+1:]
					if containsAny(line, usageMarkers) {
						lastUsageLine = make([]byte, len(line))
						copy(lastUsageLine, line)
					}
				}
			}
			if err != nil {
				// Check remaining partial for usage
				if len(partial) > 0 && containsAny(partial, usageMarkers) {
					lastUsageLine = make([]byte, len(partial))
					copy(lastUsageLine, partial)
				}
				break
			}
		}

		if len(lastUsageLine) > 0 {
			lineCopy := lastUsageLine
			go h.recordStreamingTokens(userID, model, lineCopy, requestID, requestStart)
		}
	})

	return nil
}

func (h *Handler) getUserAndModel(c *fiber.Ctx) (string, string) {
	var userID, model string
	if u := auth.GetUser(c); u != nil {
		userID = u.UserID
	}
	if m, ok := c.Locals("request_model").(string); ok {
		model = m
	}
	return userID, model
}

func (h *Handler) recordNonStreamingTokens(userID, model string, body []byte, requestID string) {
	if userID == "" {
		return
	}
	usage := ExtractUsage(body)
	h.collector.RecordTokens(userID, model, usage.PromptTokens, usage.CompletionTokens)

	if requestID == "" {
		return
	}

	rowsAffected, err := models.UpdateAuditLogTokens(h.db, requestID, usage.PromptTokens, usage.CompletionTokens)
	if err != nil {
		log.Printf("failed to update audit log tokens: %v", err)
		return
	}
	if rowsAffected > 0 {
		return
	}

	// Row not yet inserted by audit batch worker — retry
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)
		rowsAffected, err = models.UpdateAuditLogTokens(h.db, requestID, usage.PromptTokens, usage.CompletionTokens)
		if err != nil {
			log.Printf("failed to update audit log tokens (retry %d): %v", i+1, err)
			return
		}
		if rowsAffected > 0 {
			return
		}
	}
	log.Printf("audit log row not found after retries for request_id=%s", requestID)
}

func (h *Handler) recordStreamingTokens(userID, model string, line []byte, requestID string, requestStart time.Time) {
	if userID == "" {
		return
	}
	usage, ok := ExtractUsageFromSSE(line)
	if !ok {
		return
	}
	h.collector.RecordTokens(userID, model, usage.PromptTokens, usage.CompletionTokens)

	if requestID == "" || requestStart.IsZero() {
		return
	}

	latencyMs := time.Since(requestStart).Milliseconds()
	rowsAffected, err := models.UpdateAuditLogTokensAndLatency(h.db, requestID, usage.PromptTokens, usage.CompletionTokens, latencyMs)
	if err != nil {
		log.Printf("failed to update audit log tokens: %v", err)
		return
	}
	if rowsAffected > 0 {
		return
	}

	// Row not yet inserted by audit batch worker — retry
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)
		rowsAffected, err = models.UpdateAuditLogTokensAndLatency(h.db, requestID, usage.PromptTokens, usage.CompletionTokens, latencyMs)
		if err != nil {
			log.Printf("failed to update audit log tokens (retry %d): %v", i+1, err)
			return
		}
		if rowsAffected > 0 {
			return
		}
	}
	log.Printf("audit log row not found after retries for request_id=%s", requestID)
}

func containsAny(data []byte, markers [][]byte) bool {
	for _, m := range markers {
		if bytes.Contains(data, m) {
			return true
		}
	}
	return false
}
