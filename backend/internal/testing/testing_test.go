package testing

import (
	"testing"

	"itopshub/backend/internal/database"
)

func newTestRepo(t *testing.T) (*Repository, func(ticketID, id string)) {
	t.Helper()
	db, err := database.Open(t.TempDir()+"/test.db", nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	seedUser := func(ticketID, id string) {
		db.Exec(`INSERT INTO users (id, username, full_name, email, password_hash) VALUES (?, ?, ?, ?, 'x')`, id, id, id, id+"@example.com")
		db.Exec(`INSERT INTO tickets (id, ticket_number, title, severity, priority, status, environment, created_at, updated_at) VALUES (?, 'T2026-00001', 'Test', 'S3', 'Medium', 'FOR_UAT', 'PRODUCTION', datetime('now'), datetime('now'))`, ticketID)
	}

	return NewRepository(db), seedUser
}

func TestHasFailingOrIncompleteTests_NoTestCases_NotBlocked(t *testing.T) {
	repo, seed := newTestRepo(t)
	seed("tkt1", "u1")

	blocked, err := repo.HasFailingOrIncompleteTests("tkt1")
	if err != nil {
		t.Fatalf("HasFailingOrIncompleteTests: %v", err)
	}
	if blocked {
		t.Error("expected a ticket with zero test cases to not be blocked")
	}
}

func TestHasFailingOrIncompleteTests_NotStarted_Blocked(t *testing.T) {
	repo, seed := newTestRepo(t)
	seed("tkt1", "u1")

	_, err := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Login works", Environment: "UAT"})
	if err != nil {
		t.Fatalf("CreateTestCase: %v", err)
	}

	blocked, err := repo.HasFailingOrIncompleteTests("tkt1")
	if err != nil {
		t.Fatalf("HasFailingOrIncompleteTests: %v", err)
	}
	if !blocked {
		t.Error("expected a not-yet-executed test case to block UAT_PASSED")
	}
}

func TestHasFailingOrIncompleteTests_AllPass_NotBlocked(t *testing.T) {
	repo, seed := newTestRepo(t)
	seed("tkt1", "u1")

	tc1, _ := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Case 1", Environment: "UAT"})
	tc2, _ := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Case 2", Environment: "UAT"})

	if _, err := repo.Execute(tc1.ID, "u1", TestPass, "worked", ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := repo.Execute(tc2.ID, "u1", TestPass, "worked", ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	blocked, err := repo.HasFailingOrIncompleteTests("tkt1")
	if err != nil {
		t.Fatalf("HasFailingOrIncompleteTests: %v", err)
	}
	if blocked {
		t.Error("expected all-passing test cases to not block UAT_PASSED")
	}
}

func TestHasFailingOrIncompleteTests_OneFailed_Blocked(t *testing.T) {
	repo, seed := newTestRepo(t)
	seed("tkt1", "u1")

	tc1, _ := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Case 1", Environment: "UAT"})
	tc2, _ := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Case 2", Environment: "UAT"})

	repo.Execute(tc1.ID, "u1", TestPass, "worked", "")
	repo.Execute(tc2.ID, "u1", TestFail, "broke", "reproduced consistently")

	blocked, err := repo.HasFailingOrIncompleteTests("tkt1")
	if err != nil {
		t.Fatalf("HasFailingOrIncompleteTests: %v", err)
	}
	if !blocked {
		t.Error("expected one failed test case (even with another passing) to block UAT_PASSED")
	}
}

func TestExecute_RecordsTesterAndTimestamp(t *testing.T) {
	repo, seed := newTestRepo(t)
	seed("tkt1", "u1")

	tc, _ := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Case 1", Environment: "UAT"})

	updated, err := repo.Execute(tc.ID, "u1", TestPass, "as expected", "looks good")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if updated.TesterID == nil || *updated.TesterID != "u1" {
		t.Error("expected tester to be recorded")
	}
	if updated.ExecutedAt == nil {
		t.Error("expected executedAt to be stamped")
	}
	if updated.ActualResult != "as expected" {
		t.Errorf("expected actual result to be saved, got %q", updated.ActualResult)
	}
}

func TestExecute_RejectsNotStartedAsTargetStatus(t *testing.T) {
	repo, seed := newTestRepo(t)
	seed("tkt1", "u1")
	tc, _ := repo.CreateTestCase(CreateTestCaseInput{TicketID: "tkt1", Description: "Case 1", Environment: "UAT"})

	_, err := repo.Execute(tc.ID, "u1", TestNotStarted, "", "")
	if err == nil {
		t.Error("expected an error when trying to 'execute' a test back to NOT_STARTED")
	}
}
