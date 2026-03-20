package admin

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hkdb/lmgate/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

type logResponse struct {
	ID               int64  `json:"id"`
	Timestamp        string `json:"timestamp"`
	UserEmail        string `json:"user_email"`
	Action           string `json:"action"`
	Resource         string `json:"resource"`
	Detail           string `json:"detail"`
	IPAddress        string `json:"ip_address"`
	Status           string `json:"status"`
	Streaming        bool   `json:"streaming"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
	GatewayOverheadUs int64 `json:"gateway_overhead_us"`
	UpstreamTTFBMs   int64  `json:"upstream_ttfb_ms"`
	Model            string `json:"model"`
	LogType          string `json:"log_type"`
	XRealIP          string `json:"x_real_ip"`
	XForwardedFor    string `json:"x_forwarded_for"`
}

func (a *Admin) StreamLogs(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	id, ch := a.Notifier.Subscribe()

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		defer a.Notifier.Unsubscribe(id)

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

func (a *Admin) GetLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page", "25"))
	if perPage < 1 {
		perPage = 25
	}

	var conditions []string
	var args []any

	if uid := c.Query("user_id"); uid != "" {
		conditions = append(conditions, "a.user_id = ?")
		args = append(args, uid)
	}

	if action := c.Query("action"); action != "" {
		conditions = append(conditions, "a.method = ?")
		args = append(args, mapActionToMethod(action))
	}

	if status := c.Query("status"); status != "" {
		conditions = append(conditions, statusCondition(status))
	}

	if logType := c.Query("log_type"); logType != "" {
		conditions = append(conditions, "a.log_type = ?")
		args = append(args, logType)
	}

	if afterID := c.Query("after_id"); afterID != "" {
		conditions = append(conditions, "a.id > ?")
		args = append(args, afterID)
	}

	if q := c.Query("q"); q != "" {
		conditions = append(conditions, "(a.path LIKE ? OR a.method LIKE ? OR u.email LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs a LEFT JOIN users u ON a.user_id = u.id" + where
	if err := a.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query logs"})
	}

	offset := (page - 1) * perPage
	query := `SELECT a.id, a.method, a.path, a.status_code, a.ip_addr, a.created_at, a.streaming,
			a.latency_ms, a.gateway_overhead_us, a.upstream_ttfb_ms, a.prompt_tokens, a.completion_tokens,
			COALESCE(a.model, ''), COALESCE(u.email, ''), COALESCE(a.log_type, 'api'),
			a.x_real_ip, a.x_forwarded_for
		FROM audit_logs a LEFT JOIN users u ON a.user_id = u.id` +
		where + " ORDER BY a.id DESC LIMIT ? OFFSET ?"
	args = append(args, perPage, offset)

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query logs"})
	}
	defer rows.Close()

	var logs []logResponse
	for rows.Next() {
		var (
			id                                        int64
			method, path, createdAt, email, modelName  string
			logType                                   string
			statusCode                                int
			ipAddr, xRealIP, xForwardedFor            sql.NullString
			streaming                                 bool
			latencyMs, gatewayOverheadUs, upstreamTTFB int64
			promptTokens, completionTokens            int64
		)
		if err := rows.Scan(&id, &method, &path, &statusCode, &ipAddr, &createdAt, &streaming,
			&latencyMs, &gatewayOverheadUs, &upstreamTTFB, &promptTokens, &completionTokens,
			&modelName, &email, &logType, &xRealIP, &xForwardedFor); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query logs"})
		}

		ip := ""
		if ipAddr.Valid {
			ip = ipAddr.String
		}

		resp := logResponse{
			ID:               id,
			Timestamp:        createdAt,
			UserEmail:        email,
			Action:           method,
			Resource:         path,
			Detail:           fmt.Sprintf("%s %s → %d", method, path, statusCode),
			IPAddress:        ip,
			Status:           mapStatusCode(statusCode),
			Streaming:        streaming,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			LatencyMs:        latencyMs,
			GatewayOverheadUs: gatewayOverheadUs,
			UpstreamTTFBMs:   upstreamTTFB,
			Model:            modelName,
			LogType:          logType,
		}
		if xRealIP.Valid {
			resp.XRealIP = xRealIP.String
		}
		if xForwardedFor.Valid {
			resp.XForwardedFor = xForwardedFor.String
		}
		logs = append(logs, resp)
	}
	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query logs"})
	}

	if logs == nil {
		logs = []logResponse{}
	}
	return c.JSON(fiber.Map{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

func (a *Admin) DeleteAllLogs(c *fiber.Ctx) error {
	// Require confirmation query parameter to prevent accidental deletion
	if c.Query("confirm") != "true" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "add ?confirm=true to confirm bulk deletion"})
	}

	a.Collector.Reset()
	middleware.ResetAuditBuffer()

	if _, err := a.DB.Exec("DELETE FROM audit_logs"); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete audit logs"})
	}
	if _, err := a.DB.Exec("DELETE FROM usage_metrics"); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete usage metrics"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *Admin) ExportLogs(c *fiber.Ctx) error {
	format := c.Query("format", "csv")
	if format != "csv" && format != "log" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "format must be csv or log"})
	}

	var conditions []string
	var args []any

	if uid := c.Query("user_id"); uid != "" {
		conditions = append(conditions, "a.user_id = ?")
		args = append(args, uid)
	}
	if action := c.Query("action"); action != "" {
		conditions = append(conditions, "a.method = ?")
		args = append(args, mapActionToMethod(action))
	}
	if status := c.Query("status"); status != "" {
		conditions = append(conditions, statusCondition(status))
	}
	if logType := c.Query("log_type"); logType != "" {
		conditions = append(conditions, "a.log_type = ?")
		args = append(args, logType)
	}
	if q := c.Query("q"); q != "" {
		conditions = append(conditions, "(a.path LIKE ? OR a.method LIKE ? OR u.email LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	const maxRows = 50000
	query := `SELECT a.id, a.method, a.path, a.status_code, a.ip_addr, a.created_at, a.streaming,
			a.latency_ms, a.gateway_overhead_us, a.upstream_ttfb_ms, a.prompt_tokens, a.completion_tokens,
			COALESCE(a.model, ''), COALESCE(u.email, ''), COALESCE(a.log_type, 'api'),
			a.x_real_ip, a.x_forwarded_for
		FROM audit_logs a LEFT JOIN users u ON a.user_id = u.id` +
		where + " ORDER BY a.id DESC LIMIT ?"
	args = append(args, maxRows)

	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to query logs"})
	}
	defer rows.Close()

	type exportRow struct {
		id                                        int64
		method, path, createdAt, email, modelName string
		logType                                   string
		statusCode                                int
		ipAddr, xRealIP, xForwardedFor            sql.NullString
		streaming                                 bool
		latencyMs, gatewayOverheadUs, upstreamTTFB int64
		promptTokens, completionTokens            int64
	}

	var entries []exportRow
	for rows.Next() {
		var r exportRow
		if err := rows.Scan(&r.id, &r.method, &r.path, &r.statusCode, &r.ipAddr, &r.createdAt, &r.streaming,
			&r.latencyMs, &r.gatewayOverheadUs, &r.upstreamTTFB, &r.promptTokens, &r.completionTokens,
			&r.modelName, &r.email, &r.logType, &r.xRealIP, &r.xForwardedFor); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to scan logs"})
		}
		entries = append(entries, r)
	}
	if err := rows.Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to iterate logs"})
	}

	switch format {
	case "csv":
		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", `attachment; filename="audit_logs.csv"`)

		w := csv.NewWriter(c.Response().BodyWriter())
		w.Write([]string{"Timestamp", "Log Type", "Status", "Action", "Resource", "User", "IP", "X-Real-IP", "X-Forwarded-For", "Streaming", "Model",
			"Latency(ms)", "Gateway Overhead(µs)", "Upstream TTFB(ms)", "Prompt Tokens", "Completion Tokens", "Detail"})

		for _, r := range entries {
			ip := ""
			if r.ipAddr.Valid {
				ip = r.ipAddr.String
			}
			xRealIP := ""
			if r.xRealIP.Valid {
				xRealIP = r.xRealIP.String
			}
			xForwardedFor := ""
			if r.xForwardedFor.Valid {
				xForwardedFor = r.xForwardedFor.String
			}
			streaming := "false"
			if r.streaming {
				streaming = "true"
			}
			w.Write([]string{
				r.createdAt,
				r.logType,
				mapStatusCode(r.statusCode),
				r.method,
				r.path,
				r.email,
				ip,
				xRealIP,
				xForwardedFor,
				streaming,
				r.modelName,
				strconv.FormatInt(r.latencyMs, 10),
				strconv.FormatInt(r.gatewayOverheadUs, 10),
				strconv.FormatInt(r.upstreamTTFB, 10),
				strconv.FormatInt(r.promptTokens, 10),
				strconv.FormatInt(r.completionTokens, 10),
				fmt.Sprintf("%s %s → %d", r.method, r.path, r.statusCode),
			})
		}
		w.Flush()
		return w.Error()

	default: // "log"
		c.Set("Content-Type", "text/plain")
		c.Set("Content-Disposition", `attachment; filename="audit_logs.log"`)

		var buf strings.Builder
		for _, r := range entries {
			ip := ""
			if r.ipAddr.Valid {
				ip = r.ipAddr.String
			}
			model := ""
			if r.modelName != "" {
				model = " model=" + r.modelName
			}
			proxyHeaders := ""
			if r.xRealIP.Valid && r.xRealIP.String != "" {
				proxyHeaders += " x_real_ip=" + r.xRealIP.String
			}
			if r.xForwardedFor.Valid && r.xForwardedFor.String != "" {
				proxyHeaders += " x_forwarded_for=" + r.xForwardedFor.String
			}
			fmt.Fprintf(&buf, "[%s] [%s] [%s] %s %s %s%s latency=%dms tokens=%d/%d ip=%s%s - %s %s → %d\n",
				r.createdAt,
				strings.ToUpper(r.logType),
				strings.ToUpper(mapStatusCode(r.statusCode)),
				r.method,
				r.path,
				r.email,
				model,
				r.latencyMs,
				r.promptTokens,
				r.completionTokens,
				ip,
				proxyHeaders,
				r.method,
				r.path,
				r.statusCode,
			)
		}
		return c.SendString(buf.String())
	}
}

func mapStatusCode(code int) string {
	switch {
	case code >= 200 && code < 400:
		return "success"
	case code >= 400 && code < 500:
		return "error"
	case code >= 500:
		return "warning"
	default:
		return "success"
	}
}

func mapActionToMethod(action string) string {
	switch action {
	case "create":
		return "POST"
	case "update":
		return "PUT"
	case "delete":
		return "DELETE"
	case "request", "login", "logout":
		return "GET"
	default:
		return strings.ToUpper(action)
	}
}

func statusCondition(status string) string {
	switch status {
	case "success":
		return "a.status_code >= 200 AND a.status_code < 400"
	case "error":
		return "a.status_code >= 400 AND a.status_code < 500"
	case "warning":
		return "a.status_code >= 500"
	default:
		return "1=1"
	}
}
