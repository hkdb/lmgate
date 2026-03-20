package metrics

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/hkdb/lmgate/internal/database"
	"github.com/hkdb/lmgate/internal/models"
)

func testOpenDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createTestUser(t *testing.T, db *sql.DB) string {
	t.Helper()
	u, err := models.CreateUser(db, "metrics@example.com", "M", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}

func TestCollector_Record_And_Flush(t *testing.T) {
	db := testOpenDB(t)
	userID := createTestUser(t, db)

	c := &Collector{
		db:       db,
		counters: make(map[metricKey]*metricValue),
		done:     make(chan struct{}),
	}

	c.Record(userID, "llama3", 10, 20)
	c.Record(userID, "llama3", 5, 15)
	c.Flush()

	var totalPrompt, totalCompletion, totalRequests int64
	err := db.QueryRow(
		`SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(request_count),0) FROM usage_metrics`,
	).Scan(&totalPrompt, &totalCompletion, &totalRequests)
	if err != nil {
		t.Fatalf("querying metrics: %v", err)
	}

	if totalPrompt != 15 {
		t.Errorf("totalPrompt: got %d, want 15", totalPrompt)
	}
	if totalCompletion != 35 {
		t.Errorf("totalCompletion: got %d, want 35", totalCompletion)
	}
	if totalRequests != 2 {
		t.Errorf("totalRequests: got %d, want 2", totalRequests)
	}
}

func TestCollector_RecordTokens_NoRequestCount(t *testing.T) {
	db := testOpenDB(t)

	u, err := models.CreateUser(db, "tokens@example.com", "T", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	c := &Collector{
		db:       db,
		counters: make(map[metricKey]*metricValue),
		done:     make(chan struct{}),
	}

	c.RecordTokens(u.ID, "llama3", 10, 20)
	c.Flush()

	var totalRequests int64
	err = db.QueryRow(`SELECT COALESCE(SUM(request_count),0) FROM usage_metrics`).Scan(&totalRequests)
	if err != nil {
		t.Fatalf("querying metrics: %v", err)
	}

	if totalRequests != 0 {
		t.Errorf("totalRequests: got %d, want 0", totalRequests)
	}
}

func TestCollector_Reset_ClearsCounters(t *testing.T) {
	db := testOpenDB(t)

	u, err := models.CreateUser(db, "reset@example.com", "R", "pass", "user", "local", "", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	c := &Collector{
		db:       db,
		counters: make(map[metricKey]*metricValue),
		done:     make(chan struct{}),
	}

	c.Record(u.ID, "llama3", 10, 20)
	c.Reset()
	c.Flush()

	var count int64
	err = db.QueryRow(`SELECT COUNT(*) FROM usage_metrics`).Scan(&count)
	if err != nil {
		t.Fatalf("querying metrics: %v", err)
	}

	if count != 0 {
		t.Errorf("expected no metrics after reset+flush, got %d", count)
	}

	_ = time.Second // used in package, avoid unused import
}
