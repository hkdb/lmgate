package models

import (
	"testing"
)

func TestInsertAuditLogs_And_Query(t *testing.T) {
	db := testOpenDB(t)

	logs := []AuditLog{
		{Method: "GET", Path: "/v1/models", StatusCode: 200, LogType: "api"},
		{Method: "POST", Path: "/v1/chat/completions", StatusCode: 200, LogType: "api"},
	}

	if err := InsertAuditLogs(db, logs); err != nil {
		t.Fatalf("InsertAuditLogs: %v", err)
	}

	results, total, err := QueryAuditLogs(db, AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("QueryAuditLogs: %v", err)
	}
	if total != 2 {
		t.Errorf("total: got %d, want 2", total)
	}
	if len(results) != 2 {
		t.Errorf("results: got %d, want 2", len(results))
	}
}

func TestQueryAuditLogs_FilterByLogType(t *testing.T) {
	db := testOpenDB(t)

	logs := []AuditLog{
		{Method: "GET", Path: "/api", StatusCode: 200, LogType: "api"},
		{Method: "GET", Path: "/admin", StatusCode: 200, LogType: "admin"},
		{Method: "POST", Path: "/login", StatusCode: 401, LogType: "security"},
	}

	if err := InsertAuditLogs(db, logs); err != nil {
		t.Fatalf("InsertAuditLogs: %v", err)
	}

	results, total, err := QueryAuditLogs(db, AuditFilter{LogType: "security", Limit: 10})
	if err != nil {
		t.Fatalf("QueryAuditLogs: %v", err)
	}
	if total != 1 {
		t.Errorf("total: got %d, want 1", total)
	}
	if len(results) != 1 {
		t.Errorf("results: got %d, want 1", len(results))
	}
}

func TestQueryAuditLogs_Pagination(t *testing.T) {
	db := testOpenDB(t)

	logs := make([]AuditLog, 5)
	for i := range logs {
		logs[i] = AuditLog{Method: "GET", Path: "/test", StatusCode: 200, LogType: "api"}
	}

	if err := InsertAuditLogs(db, logs); err != nil {
		t.Fatalf("InsertAuditLogs: %v", err)
	}

	results, total, err := QueryAuditLogs(db, AuditFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("QueryAuditLogs: %v", err)
	}
	if total != 5 {
		t.Errorf("total: got %d, want 5", total)
	}
	if len(results) != 2 {
		t.Errorf("results: got %d, want 2", len(results))
	}
}
