package dashboard

import (
	"database/sql"
	"testing"

	"itopshub/backend/internal/database"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(t.TempDir()+"/test.db", nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO users (id, username, full_name, email, password_hash) VALUES (?, ?, ?, ?, 'x')`,
		id, id, id, id+"@example.com",
	)
	if err != nil {
		t.Fatalf("seed user %s: %v", id, err)
	}
}

// createTicket inserts a minimal ticket directly (bypassing the tickets
// package to avoid an import cycle) with the given severity/status/assignee,
// and seeds a STARTED sla_event so the SLA engine can compute against it.
func createTicket(t *testing.T, db *sql.DB, id, number, severity, status string, assigneeID *string, createdAgoMinutes int) {
	t.Helper()
	policyID := map[string]string{"S1": "sla-s1", "S2": "sla-s2", "S3": "sla-s3", "S4": "sla-s4"}[severity]

	_, err := db.Exec(`
		INSERT INTO tickets (id, ticket_number, title, requester_id, assignee_id, severity, priority, status, environment, sla_policy_id, created_at, updated_at)
		VALUES (?, ?, 'Test ticket', NULL, ?, ?, 'Medium', ?, 'PRODUCTION', ?, datetime('now', ? || ' minutes'), datetime('now'))
	`, id, number, assigneeID, severity, status, policyID, -createdAgoMinutes)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	_, err = db.Exec(
		`INSERT INTO sla_events (id, ticket_id, event_type, created_at) VALUES (?, ?, 'STARTED', datetime('now', ? || ' minutes'))`,
		"slaevt-"+id, id, -createdAgoMinutes,
	)
	if err != nil {
		t.Fatalf("seed sla event: %v", err)
	}
}

func TestComputeAttention_BucketsBySLAStatus(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")

	// S1 = 240 min resolution. 200 min elapsed = 83% = CRITICAL.
	createTicket(t, db, "t1", "T2026-00001", "S1", "IN_PROGRESS", strPtr("u1"), 200)
	// S1, 10 min elapsed = ~4% = WITHIN_SLA (not in any bucket).
	createTicket(t, db, "t2", "T2026-00002", "S1", "NEW", nil, 10)
	// S1, 300 min elapsed = 125% = BREACHED.
	createTicket(t, db, "t3", "T2026-00003", "S1", "ACKNOWLEDGED", nil, 300)

	result, err := ComputeAttention(db, "u1")
	if err != nil {
		t.Fatalf("ComputeAttention: %v", err)
	}

	if len(result.SLACritical) != 1 || result.SLACritical[0].ID != "t1" {
		t.Errorf("expected t1 in SLACritical, got %+v", result.SLACritical)
	}
	if len(result.SLABreached) != 1 || result.SLABreached[0].ID != "t3" {
		t.Errorf("expected t3 in SLABreached, got %+v", result.SLABreached)
	}
	if result.WaitingForMyAction != 1 {
		t.Errorf("expected 1 ticket waiting for u1's action (t1, assigned), got %d", result.WaitingForMyAction)
	}
}

func TestComputeAttention_StatusCounts(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")

	createTicket(t, db, "t1", "T2026-00001", "S3", "IN_PROGRESS", nil, 5)
	createTicket(t, db, "t2", "T2026-00002", "S3", "FOR_UAT", nil, 5)
	createTicket(t, db, "t3", "T2026-00003", "S3", "FOR_PRE_PROD", nil, 5)
	createTicket(t, db, "t4", "T2026-00004", "S3", "FOR_PRODUCTION", nil, 5)

	result, err := ComputeAttention(db, "u1")
	if err != nil {
		t.Fatalf("ComputeAttention: %v", err)
	}

	if result.WaitingForDevelopment != 1 {
		t.Errorf("expected 1 in-progress ticket, got %d", result.WaitingForDevelopment)
	}
	if result.ReadyForUAT != 1 {
		t.Errorf("expected 1 FOR_UAT ticket, got %d", result.ReadyForUAT)
	}
	if result.ReadyForPreProd != 1 {
		t.Errorf("expected 1 FOR_PRE_PROD ticket, got %d", result.ReadyForPreProd)
	}
	if result.ReadyForProduction != 1 {
		t.Errorf("expected 1 FOR_PRODUCTION ticket, got %d", result.ReadyForProduction)
	}
}

func TestComputePriorityQueue_RanksBreachedS1AboveFreshS4(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")

	// Breached S1 (300/240 min = 125%).
	createTicket(t, db, "urgent", "T2026-00001", "S1", "IN_PROGRESS", strPtr("u1"), 300)
	// Fresh S4 (barely any time elapsed).
	createTicket(t, db, "low", "T2026-00002", "S4", "NEW", strPtr("u1"), 2)

	items, err := ComputePriorityQueue(db, "u1", false, 10)
	if err != nil {
		t.Fatalf("ComputePriorityQueue: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "urgent" {
		t.Errorf("expected breached S1 ranked first, got %s (score %v) vs %s (score %v)",
			items[0].ID, items[0].Score, items[1].ID, items[1].Score)
	}
	if items[0].Rank != 1 || items[1].Rank != 2 {
		t.Errorf("expected ranks 1 and 2, got %d and %d", items[0].Rank, items[1].Rank)
	}
}

func TestComputePriorityQueue_ScopeMineExcludesOthers(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")
	seedUser(t, db, "u2")

	createTicket(t, db, "mine", "T2026-00001", "S2", "IN_PROGRESS", strPtr("u1"), 10)
	createTicket(t, db, "theirs", "T2026-00002", "S1", "IN_PROGRESS", strPtr("u2"), 10)

	items, err := ComputePriorityQueue(db, "u1", false, 10)
	if err != nil {
		t.Fatalf("ComputePriorityQueue: %v", err)
	}
	if len(items) != 1 || items[0].ID != "mine" {
		t.Errorf("expected only u1's ticket in scoped queue, got %+v", items)
	}

	all, err := ComputePriorityQueue(db, "u1", true, 10)
	if err != nil {
		t.Fatalf("ComputePriorityQueue (scope=all): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected both tickets with scope=all, got %d", len(all))
	}
}

func TestComputeMyWork_UrgentRequiresSeverityAndSLA(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "u1")

	// S1 + breached -> urgent.
	createTicket(t, db, "urgent1", "T2026-00001", "S1", "IN_PROGRESS", strPtr("u1"), 300)
	// S4 + breached (S4 resolution is much longer, so this won't actually be breached this soon) -> not urgent regardless.
	createTicket(t, db, "notsev", "T2026-00002", "S4", "IN_PROGRESS", strPtr("u1"), 10)
	// S1 but within SLA -> not urgent (severity alone isn't enough).
	createTicket(t, db, "notsla", "T2026-00003", "S1", "IN_PROGRESS", strPtr("u1"), 5)

	result, err := ComputeMyWork(db, "u1")
	if err != nil {
		t.Fatalf("ComputeMyWork: %v", err)
	}

	if len(result.Urgent) != 1 || result.Urgent[0].ID != "urgent1" {
		t.Errorf("expected only urgent1 in Urgent, got %+v", result.Urgent)
	}
	if len(result.AssignedToMe) != 3 {
		t.Errorf("expected all 3 tickets in AssignedToMe, got %d", len(result.AssignedToMe))
	}
}

func strPtr(s string) *string { return &s }
