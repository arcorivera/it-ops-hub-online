package incidents

import (
	"testing"

	"itopshub/backend/internal/database"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	db, err := database.Open(t.TempDir()+"/test.db", nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewRepository(db)
}

// fakeRCAChecker lets tests control IsRCACompleted without depending on the
// rca package (avoids a test-only import cycle risk and keeps this package
// tested in isolation).
type fakeRCAChecker struct{ completed bool }

func (f *fakeRCAChecker) IsRCACompleted(incidentID string) (bool, error) {
	return f.completed, nil
}

func TestIncidentNumberGenerated(t *testing.T) {
	repo := newTestRepo(t)
	inc, err := repo.Create(CreateInput{Title: "DB outage", Severity: "S1", Environment: "PRODUCTION"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inc.IncidentNumber == "" {
		t.Error("expected a generated incident number")
	}
	if inc.Status != StatusOpen {
		t.Errorf("expected OPEN, got %s", inc.Status)
	}
}

func TestIncidentNumberSequenceIncrements(t *testing.T) {
	repo := newTestRepo(t)
	i1, _ := repo.Create(CreateInput{Title: "First", Severity: "S3", Environment: "PRODUCTION"})
	i2, _ := repo.Create(CreateInput{Title: "Second", Severity: "S3", Environment: "PRODUCTION"})
	if i1.IncidentNumber == i2.IncidentNumber {
		t.Errorf("expected distinct incident numbers, got %s twice", i1.IncidentNumber)
	}
}

func TestRequiresRCA(t *testing.T) {
	cases := map[string]bool{"S1": true, "S2": true, "S3": false, "S4": false}
	for sev, want := range cases {
		if got := RequiresRCA(sev); got != want {
			t.Errorf("RequiresRCA(%s) = %v, want %v", sev, got, want)
		}
	}
}

func TestUpdateStatus_ValidTransition(t *testing.T) {
	repo := newTestRepo(t)
	inc, _ := repo.Create(CreateInput{Title: "Incident", Severity: "S3", Environment: "PRODUCTION"})

	updated, err := repo.UpdateStatus(inc.ID, StatusInvestigating, false, nil)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != StatusInvestigating {
		t.Errorf("expected INVESTIGATING, got %s", updated.Status)
	}
}

func TestUpdateStatus_InvalidTransitionRejected(t *testing.T) {
	repo := newTestRepo(t)
	inc, _ := repo.Create(CreateInput{Title: "Incident", Severity: "S3", Environment: "PRODUCTION"})

	// OPEN -> CLOSED directly is not a valid transition.
	_, err := repo.UpdateStatus(inc.ID, StatusClosed, false, nil)
	if err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestUpdateStatus_S1ClosureBlockedWithoutRCA(t *testing.T) {
	repo := newTestRepo(t)
	inc, _ := repo.Create(CreateInput{Title: "Critical outage", Severity: "S1", Environment: "PRODUCTION"})

	repo.UpdateStatus(inc.ID, StatusInvestigating, false, nil)
	repo.UpdateStatus(inc.ID, StatusMitigated, false, nil)
	repo.UpdateStatus(inc.ID, StatusResolved, false, nil)

	checker := &fakeRCAChecker{completed: false}
	_, err := repo.UpdateStatus(inc.ID, StatusClosed, false, checker)
	if err != ErrRCARequired {
		t.Errorf("expected ErrRCARequired for S1 closure without completed RCA, got %v", err)
	}
}

func TestUpdateStatus_S1ClosureAllowedWithCompletedRCA(t *testing.T) {
	repo := newTestRepo(t)
	inc, _ := repo.Create(CreateInput{Title: "Critical outage", Severity: "S1", Environment: "PRODUCTION"})

	repo.UpdateStatus(inc.ID, StatusInvestigating, false, nil)
	repo.UpdateStatus(inc.ID, StatusMitigated, false, nil)
	repo.UpdateStatus(inc.ID, StatusResolved, false, nil)

	checker := &fakeRCAChecker{completed: true}
	updated, err := repo.UpdateStatus(inc.ID, StatusClosed, false, checker)
	if err != nil {
		t.Fatalf("expected closure to succeed with completed RCA, got %v", err)
	}
	if updated.Status != StatusClosed {
		t.Errorf("expected CLOSED, got %s", updated.Status)
	}
}

func TestUpdateStatus_S3ClosureNeverGated(t *testing.T) {
	repo := newTestRepo(t)
	inc, _ := repo.Create(CreateInput{Title: "Minor issue", Severity: "S3", Environment: "PRODUCTION"})

	repo.UpdateStatus(inc.ID, StatusInvestigating, false, nil)
	repo.UpdateStatus(inc.ID, StatusMitigated, false, nil)
	repo.UpdateStatus(inc.ID, StatusResolved, false, nil)

	checker := &fakeRCAChecker{completed: false}
	updated, err := repo.UpdateStatus(inc.ID, StatusClosed, false, checker)
	if err != nil {
		t.Fatalf("expected S3 closure to never be RCA-gated, got %v", err)
	}
	if updated.Status != StatusClosed {
		t.Errorf("expected CLOSED, got %s", updated.Status)
	}
}

func TestUpdateStatus_ForceOverridesRCAGate(t *testing.T) {
	repo := newTestRepo(t)
	inc, _ := repo.Create(CreateInput{Title: "Critical outage", Severity: "S2", Environment: "PRODUCTION"})

	repo.UpdateStatus(inc.ID, StatusInvestigating, false, nil)
	repo.UpdateStatus(inc.ID, StatusMitigated, false, nil)
	repo.UpdateStatus(inc.ID, StatusResolved, false, nil)

	checker := &fakeRCAChecker{completed: false}
	updated, err := repo.UpdateStatus(inc.ID, StatusClosed, true, checker)
	if err != nil {
		t.Fatalf("expected forced closure to bypass the RCA gate, got %v", err)
	}
	if updated.Status != StatusClosed {
		t.Errorf("expected CLOSED, got %s", updated.Status)
	}
}

func TestCreate_LinkedTicketFlaggedRCARequired(t *testing.T) {
	repo := newTestRepo(t)
	db := repo.db

	db.Exec(`INSERT INTO users (id, username, full_name, email, password_hash) VALUES ('u1','u1','u1','u1@example.com','x')`)
	db.Exec(`INSERT INTO tickets (id, ticket_number, title, severity, priority, status, environment, created_at, updated_at)
		VALUES ('tkt1', 'T2026-00001', 'Linked ticket', 'S1', 'Critical', 'IN_PROGRESS', 'PRODUCTION', datetime('now'), datetime('now'))`)

	ticketID := "tkt1"
	_, err := repo.Create(CreateInput{Title: "Outage", Severity: "S1", Environment: "PRODUCTION", TicketID: &ticketID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var rcaRequired int
	db.QueryRow(`SELECT rca_required FROM tickets WHERE id = 'tkt1'`).Scan(&rcaRequired)
	if rcaRequired != 1 {
		t.Error("expected linked ticket to be flagged rca_required=1 for an S1 incident")
	}
}
