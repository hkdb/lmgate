package models

import (
	"database/sql"
)

type UsageMetric struct {
	ID               int64  `json:"id"`
	UserID           string `json:"user_id"`
	Model            string `json:"model"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	RequestCount     int64  `json:"request_count"`
	PeriodStart      string `json:"period_start"`
	PeriodEnd        string `json:"period_end"`
}

type UsageSummary struct {
	TotalRequests        int64          `json:"total_requests"`
	TotalPromptTokens    int64          `json:"total_prompt_tokens"`
	TotalCompletionTokens int64         `json:"total_completion_tokens"`
	UniqueUsers          int64          `json:"unique_users"`
	UniqueModels         int64          `json:"unique_models"`
	ByModel              []ModelUsage   `json:"by_model"`
	TimeSeries           []TimeSeriesPoint `json:"time_series"`
}

type ModelUsage struct {
	Model            string `json:"model"`
	RequestCount     int64  `json:"request_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

type TimeSeriesPoint struct {
	Period       string `json:"period"`
	RequestCount int64  `json:"request_count"`
}

func UpsertUsageMetric(db *sql.DB, m UsageMetric) error {
	_, err := db.Exec(
		`INSERT INTO usage_metrics (user_id, model, prompt_tokens, completion_tokens, request_count, period_start, period_end)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, model, period_start) DO UPDATE SET
		   prompt_tokens = prompt_tokens + excluded.prompt_tokens,
		   completion_tokens = completion_tokens + excluded.completion_tokens,
		   request_count = request_count + excluded.request_count`,
		m.UserID, m.Model, m.PromptTokens, m.CompletionTokens, m.RequestCount, m.PeriodStart, m.PeriodEnd,
	)
	return err
}

func GetUsageSummary(db *sql.DB, since string) (*UsageSummary, error) {
	s := &UsageSummary{}

	err := db.QueryRow(
		`SELECT COALESCE(SUM(request_count),0), COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0),
		        COUNT(DISTINCT user_id), COUNT(DISTINCT model)
		 FROM usage_metrics WHERE period_start >= ?`, since,
	).Scan(&s.TotalRequests, &s.TotalPromptTokens, &s.TotalCompletionTokens, &s.UniqueUsers, &s.UniqueModels)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(
		`SELECT model, SUM(request_count), SUM(prompt_tokens), SUM(completion_tokens)
		 FROM usage_metrics WHERE period_start >= ?
		 GROUP BY model ORDER BY SUM(request_count) DESC`, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var m ModelUsage
		if err := rows.Scan(&m.Model, &m.RequestCount, &m.PromptTokens, &m.CompletionTokens); err != nil {
			return nil, err
		}
		s.ByModel = append(s.ByModel, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tsRows, err := db.Query(
		`SELECT period_start, SUM(request_count)
		 FROM usage_metrics WHERE period_start >= ?
		 GROUP BY period_start ORDER BY period_start`, since,
	)
	if err != nil {
		return nil, err
	}
	defer tsRows.Close()

	for tsRows.Next() {
		var p TimeSeriesPoint
		if err := tsRows.Scan(&p.Period, &p.RequestCount); err != nil {
			return nil, err
		}
		s.TimeSeries = append(s.TimeSeries, p)
	}

	return s, tsRows.Err()
}

func GetUserUsage(db *sql.DB, userID, since string) ([]UsageMetric, error) {
	rows, err := db.Query(
		`SELECT id, user_id, model, prompt_tokens, completion_tokens, request_count, period_start, period_end
		 FROM usage_metrics WHERE user_id = ? AND period_start >= ?
		 ORDER BY period_start DESC`, userID, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []UsageMetric
	for rows.Next() {
		var m UsageMetric
		if err := rows.Scan(&m.ID, &m.UserID, &m.Model, &m.PromptTokens, &m.CompletionTokens, &m.RequestCount, &m.PeriodStart, &m.PeriodEnd); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}
