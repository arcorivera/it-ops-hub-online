package projects

import (
	"testing"
	"time"

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

func TestCreateProject_DuplicateKeyRejected(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.CreateProject(CreateProjectInput{ProjectKey: "IOS", Name: "iOS App"})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = repo.CreateProject(CreateProjectInput{ProjectKey: "IOS", Name: "iOS App v2"})
	if err != ErrDuplicateKey {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}
}

func TestCreateProject_DefaultsToPlanning(t *testing.T) {
	repo := newTestRepo(t)
	p, err := repo.CreateProject(CreateProjectInput{ProjectKey: "APP", Name: "App"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Status != StatusPlanning {
		t.Errorf("expected PLANNING, got %s", p.Status)
	}
}

func TestMilestone_OverdueDetection_PastDueNotCompleted(t *testing.T) {
	repo := newTestRepo(t)
	p, _ := repo.CreateProject(CreateProjectInput{ProjectKey: "APP", Name: "App"})

	pastDue := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	m, err := repo.CreateMilestone(CreateMilestoneInput{ProjectID: p.ID, Name: "Beta launch", DueDate: &pastDue})
	if err != nil {
		t.Fatalf("CreateMilestone: %v", err)
	}

	fetched, err := repo.GetMilestone(m.ID)
	if err != nil {
		t.Fatalf("GetMilestone: %v", err)
	}
	if fetched.Status != MilestoneOverdue {
		t.Errorf("expected OVERDUE for a past-due, not-started milestone, got %s", fetched.Status)
	}
}

func TestMilestone_OverdueDetection_CompletedNeverOverdue(t *testing.T) {
	repo := newTestRepo(t)
	p, _ := repo.CreateProject(CreateProjectInput{ProjectKey: "APP", Name: "App"})

	pastDue := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	m, _ := repo.CreateMilestone(CreateMilestoneInput{ProjectID: p.ID, Name: "Beta launch", DueDate: &pastDue})

	completed := MilestoneCompleted
	_, err := repo.UpdateMilestone(m.ID, UpdateMilestoneInput{Status: &completed})
	if err != nil {
		t.Fatalf("UpdateMilestone: %v", err)
	}

	fetched, err := repo.GetMilestone(m.ID)
	if err != nil {
		t.Fatalf("GetMilestone: %v", err)
	}
	if fetched.Status != MilestoneCompleted {
		t.Errorf("expected a completed milestone to stay COMPLETED even if past due, got %s", fetched.Status)
	}
}

func TestMilestone_OverdueDetection_FutureDueNotOverdue(t *testing.T) {
	repo := newTestRepo(t)
	p, _ := repo.CreateProject(CreateProjectInput{ProjectKey: "APP", Name: "App"})

	futureDue := time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02")
	m, _ := repo.CreateMilestone(CreateMilestoneInput{ProjectID: p.ID, Name: "Beta launch", DueDate: &futureDue})

	fetched, err := repo.GetMilestone(m.ID)
	if err != nil {
		t.Fatalf("GetMilestone: %v", err)
	}
	if fetched.Status != MilestoneNotStarted {
		t.Errorf("expected NOT_STARTED for a future-due milestone, got %s", fetched.Status)
	}
}

func TestGetProjectStats_CountsTicketsCorrectly(t *testing.T) {
	repo := newTestRepo(t)
	p, _ := repo.CreateProject(CreateProjectInput{ProjectKey: "APP", Name: "App"})

	db := repo.db
	db.Exec(`INSERT INTO users (id, username, full_name, email, password_hash) VALUES ('u1','u1','u1','u1@example.com','x')`)

	seedTicket := func(id, status, severity string) {
		db.Exec(`INSERT INTO tickets (id, ticket_number, title, project_id, severity, priority, status, environment, created_at, updated_at)
			VALUES (?, ?, 'T', ?, ?, 'Medium', ?, 'PRODUCTION', datetime('now'), datetime('now'))`, id, id, p.ID, severity, status)
	}
	seedTicket("t1", "IN_PROGRESS", "S1")
	seedTicket("t2", "NEW", "S3")
	seedTicket("t3", "RESOLVED", "S2")
	seedTicket("t4", "CLOSED", "S4")

	stats, err := repo.GetProjectStats(p.ID)
	if err != nil {
		t.Fatalf("GetProjectStats: %v", err)
	}
	if stats.OpenTickets != 2 {
		t.Errorf("expected 2 open tickets, got %d", stats.OpenTickets)
	}
	if stats.CriticalTickets != 1 {
		t.Errorf("expected 1 critical (S1/S2) open ticket, got %d", stats.CriticalTickets)
	}
	if stats.CompletedTickets != 2 {
		t.Errorf("expected 2 completed (resolved+closed) tickets, got %d", stats.CompletedTickets)
	}
	if stats.TotalTickets != 4 {
		t.Errorf("expected 4 total tickets, got %d", stats.TotalTickets)
	}
}
