package models

import (
	"database/sql"
	"fmt"
	"strings"
)

type AuditLog struct {
	ID         int64   `json:"id"`
	UserID     *string `json:"user_id,omitempty"`
	TokenID    *string `json:"token_id,omitempty"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Model      *string `json:"model,omitempty"`
	StatusCode int     `json:"status_code"`
	LatencyMs         int64   `json:"latency_ms"`
	GatewayOverheadUs int64   `json:"gateway_overhead_us"`
	UpstreamTTFBMs    int64   `json:"upstream_ttfb_ms"`
	Streaming         bool    `json:"streaming"`
	IPAddr            *string `json:"ip_addr,omitempty"`
	XRealIP           *string `json:"x_real_ip,omitempty"`
	XForwardedFor     *string `json:"x_forwarded_for,omitempty"`
	PromptTokens      int64   `json:"prompt_tokens"`
	CompletionTokens  int64   `json:"completion_tokens"`
	RequestID         string  `json:"request_id,omitempty"`
	LogType           string  `json:"log_type"`
	CreatedAt  string  `json:"created_at"`
}

type AuditFilter struct {
	UserID  string
	Method  string
	Path    string
	LogType string
	Limit   int
	Offset  int
}

func InsertAuditLogs(db *sql.DB, logs []AuditLog) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO audit_logs (user_id, token_id, method, path, model, status_code, latency_ms, gateway_overhead_us, upstream_ttfb_ms, streaming, ip_addr, x_real_ip, x_forwarded_for, prompt_tokens, completion_tokens, request_id, log_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, l := range logs {
		logType := l.LogType
		if logType == "" {
			logType = "api"
		}
		if _, err := stmt.Exec(l.UserID, l.TokenID, l.Method, l.Path, l.Model, l.StatusCode, l.LatencyMs, l.GatewayOverheadUs, l.UpstreamTTFBMs, l.Streaming, l.IPAddr, l.XRealIP, l.XForwardedFor, l.PromptTokens, l.CompletionTokens, l.RequestID, logType); err != nil {
			return fmt.Errorf("inserting audit log: %w", err)
		}
	}

	return tx.Commit()
}

func QueryAuditLogs(db *sql.DB, filter AuditFilter) ([]AuditLog, int, error) {
	var conditions []string
	var args []any

	if filter.UserID != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, filter.Method)
	}
	if filter.Path != "" {
		conditions = append(conditions, "path LIKE ?")
		args = append(args, "%"+filter.Path+"%")
	}
	if filter.LogType != "" {
		conditions = append(conditions, "log_type = ?")
		args = append(args, filter.LogType)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + where
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	query := "SELECT id, user_id, token_id, method, path, model, status_code, latency_ms, gateway_overhead_us, upstream_ttfb_ms, streaming, ip_addr, x_real_ip, x_forwarded_for, prompt_tokens, completion_tokens, COALESCE(log_type, 'api'), created_at FROM audit_logs" +
		where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, filter.Offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.TokenID, &l.Method, &l.Path, &l.Model, &l.StatusCode, &l.LatencyMs, &l.GatewayOverheadUs, &l.UpstreamTTFBMs, &l.Streaming, &l.IPAddr, &l.XRealIP, &l.XForwardedFor, &l.PromptTokens, &l.CompletionTokens, &l.LogType, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func UpdateAuditLogTokens(db *sql.DB, requestID string, promptTokens, completionTokens int64) (int64, error) {
	res, err := db.Exec(
		`UPDATE audit_logs SET prompt_tokens = ?, completion_tokens = ? WHERE request_id = ?`,
		promptTokens, completionTokens, requestID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func UpdateAuditLogTokensAndLatency(db *sql.DB, requestID string, promptTokens, completionTokens, latencyMs int64) (int64, error) {
	res, err := db.Exec(
		`UPDATE audit_logs SET prompt_tokens = ?, completion_tokens = ?, latency_ms = ? WHERE request_id = ?`,
		promptTokens, completionTokens, latencyMs, requestID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func PruneAuditLogsByType(db *sql.DB, apiDays, adminDays, securityDays int) (int64, error) {
	var total int64
	types := []struct {
		logType string
		days    int
	}{
		{"api", apiDays},
		{"admin", adminDays},
		{"security", securityDays},
	}
	for _, t := range types {
		if t.days <= 0 {
			continue
		}
		res, err := db.Exec(
			`DELETE FROM audit_logs WHERE log_type = ? AND created_at < datetime('now', '-' || ? || ' days')`,
			t.logType, t.days,
		)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}
