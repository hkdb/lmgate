package models

import (
	"testing"
)

func TestUpsertUsageMetric_Accumulates(t *testing.T) {
	db := testOpenDB(t)

	u, err := CreateUser(db, "usage@example.com", "U", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	m := UsageMetric{
		UserID:           u.ID,
		Model:            "llama3",
		PromptTokens:     10,
		CompletionTokens: 20,
		RequestCount:     1,
		PeriodStart:      "2025-01-01 00:00:00",
		PeriodEnd:        "2025-01-01 01:00:00",
	}

	if err := UpsertUsageMetric(db, m); err != nil {
		t.Fatalf("first UpsertUsageMetric: %v", err)
	}
	if err := UpsertUsageMetric(db, m); err != nil {
		t.Fatalf("second UpsertUsageMetric: %v", err)
	}

	summary, err := GetUsageSummary(db, "2025-01-01 00:00:00")
	if err != nil {
		t.Fatalf("GetUsageSummary: %v", err)
	}

	if summary.TotalPromptTokens != 20 {
		t.Errorf("TotalPromptTokens: got %d, want 20", summary.TotalPromptTokens)
	}
	if summary.TotalCompletionTokens != 40 {
		t.Errorf("TotalCompletionTokens: got %d, want 40", summary.TotalCompletionTokens)
	}
}

func TestGetUsageSummary_Aggregates(t *testing.T) {
	db := testOpenDB(t)

	u1, err := CreateUser(db, "agg1@example.com", "U1", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser u1: %v", err)
	}
	u2, err := CreateUser(db, "agg2@example.com", "U2", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser u2: %v", err)
	}

	metrics := []UsageMetric{
		{UserID: u1.ID, Model: "llama3", PromptTokens: 10, CompletionTokens: 20, RequestCount: 1, PeriodStart: "2025-01-01 00:00:00", PeriodEnd: "2025-01-01 01:00:00"},
		{UserID: u2.ID, Model: "mistral", PromptTokens: 30, CompletionTokens: 40, RequestCount: 2, PeriodStart: "2025-01-01 00:00:00", PeriodEnd: "2025-01-01 01:00:00"},
	}

	for _, m := range metrics {
		if err := UpsertUsageMetric(db, m); err != nil {
			t.Fatalf("UpsertUsageMetric: %v", err)
		}
	}

	summary, err := GetUsageSummary(db, "2025-01-01 00:00:00")
	if err != nil {
		t.Fatalf("GetUsageSummary: %v", err)
	}

	if summary.TotalRequests != 3 {
		t.Errorf("TotalRequests: got %d, want 3", summary.TotalRequests)
	}
	if summary.UniqueUsers != 2 {
		t.Errorf("UniqueUsers: got %d, want 2", summary.UniqueUsers)
	}
	if summary.UniqueModels != 2 {
		t.Errorf("UniqueModels: got %d, want 2", summary.UniqueModels)
	}
}
