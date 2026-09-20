package tickets

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

	// ticket_history.user_id has a foreign key to users(id); seed the actor
	// IDs used across these tests so inserts succeed exactly as they would
	// against a real session-authenticated user.
	for _, id := range []string{"actor-1", "u1", "admin-1", "someone-else", "user-42"} {
		_, err := db.Exec(
			`INSERT INTO users (id, username, full_name, email, password_hash) VALUES (?, ?, ?, ?, 'x')`,
			id, id, id, id+"@example.com",
		)
		if err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
	}

	return NewRepository(db)
}

func TestCreateTicket_AssignsNumberAndSLA(t *testing.T) {
	repo := newTestRepo(t)

	tk, err := repo.Create(CreateInput{
		Title:       "Mobile app crash",
		Description: "Crashes on launch",
		Severity:    SeverityS1,
		Priority:    PriorityCritical,
		Environment: EnvProduction,
	}, "actor-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if tk.TicketNumber == "" {
		t.Error("expected a generated ticket number")
	}
	if tk.Status != StatusNew {
		t.Errorf("expected status NEW, got %s", tk.Status)
	}
	if tk.SLAPolicyID == nil {
		t.Fatal("expected an SLA policy to be assigned for S1")
	}
	if tk.ResponseDeadline == nil || tk.ResolutionDeadline == nil {
		t.Error("expected response and resolution deadlines to be set")
	}
	if !tk.ResponseDeadline.After(tk.CreatedAt) {
		t.Error("expected response deadline to be after creation time")
	}
}

func TestTicketNumberSequenceIncrements(t *testing.T) {
	repo := newTestRepo(t)

	t1, _ := repo.Create(CreateInput{Title: "First", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")
	t2, _ := repo.Create(CreateInput{Title: "Second", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")

	if t1.TicketNumber == t2.TicketNumber {
		t.Fatalf("expected distinct ticket numbers, got %s twice", t1.TicketNumber)
	}
}

func TestStatusTransition_Valid(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")

	updated, err := repo.UpdateStatus(tk.ID, StatusAcknowledged, "u1", "", false)
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != StatusAcknowledged {
		t.Errorf("expected ACKNOWLEDGED, got %s", updated.Status)
	}
	if updated.AcknowledgedAt == nil {
		t.Error("expected acknowledgedAt to be stamped")
	}
}

func TestStatusTransition_Invalid(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")

	// NEW -> RESOLVED is not a valid direct transition.
	_, err := repo.UpdateStatus(tk.ID, StatusResolved, "u1", "", false)
	if err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestStatusTransition_ForceOverridesGraph(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")

	updated, err := repo.UpdateStatus(tk.ID, StatusResolved, "admin-1", "admin override", true)
	if err != nil {
		t.Fatalf("expected forced transition to succeed, got %v", err)
	}
	if updated.Status != StatusResolved {
		t.Errorf("expected RESOLVED, got %s", updated.Status)
	}
}

func TestCommentsLifecycle(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")

	c, err := repo.AddComment(tk.ID, "u1", "Investigating now")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	list, err := repo.ListComments(tk.ID)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(list) != 1 || list[0].Content != "Investigating now" {
		t.Errorf("expected 1 comment with content, got %+v", list)
	}

	if err := repo.DeleteComment(c.ID, "u1", false); err != nil {
		t.Fatalf("DeleteComment: %v", err)
	}
	list, _ = repo.ListComments(tk.ID)
	if len(list) != 0 {
		t.Errorf("expected 0 visible comments after delete, got %d", len(list))
	}
}

func TestDeleteComment_NotOwnerNotAdmin_Fails(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")
	c, _ := repo.AddComment(tk.ID, "u1", "mine")

	err := repo.DeleteComment(c.ID, "someone-else", false)
	if err == nil {
		t.Error("expected an authorization error, got nil")
	}
}

func TestHistoryRecordsCreationAndStatusChange(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")
	repo.UpdateStatus(tk.ID, StatusAcknowledged, "u1", "", false)

	history, err := repo.ListHistory(tk.ID)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history events (created + status change), got %d", len(history))
	}
	if history[0].EventType != "CREATED" {
		t.Errorf("expected first event CREATED, got %s", history[0].EventType)
	}
	if history[1].EventType != "STATUS_CHANGED" {
		t.Errorf("expected second event STATUS_CHANGED, got %s", history[1].EventType)
	}
}

func TestListFilterBySeverity(t *testing.T) {
	repo := newTestRepo(t)
	repo.Create(CreateInput{Title: "S1 ticket", Severity: SeverityS1, Priority: PriorityCritical, Environment: EnvProduction}, "u1")
	repo.Create(CreateInput{Title: "S4 ticket", Severity: SeverityS4, Priority: PriorityLow, Environment: EnvDev}, "u1")

	result, err := repo.List(ListFilter{Severity: []string{SeverityS1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 S1 ticket, got %d", result.Total)
	}
	if len(result.Tickets) != 1 || result.Tickets[0].Severity != SeverityS1 {
		t.Errorf("expected filtered result to only contain S1 tickets")
	}
}

func TestAssignTicket(t *testing.T) {
	repo := newTestRepo(t)
	tk, _ := repo.Create(CreateInput{Title: "T", Severity: SeverityS3, Priority: PriorityMedium, Environment: EnvDev}, "u1")

	assignee := "user-42"
	updated, err := repo.Assign(tk.ID, &assignee, "u1")
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if updated.AssigneeID == nil || *updated.AssigneeID != assignee {
		t.Errorf("expected assignee %s, got %v", assignee, updated.AssigneeID)
	}
}

func TestIsValidTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{StatusNew, StatusAcknowledged, true},
		{StatusNew, StatusResolved, false},
		{StatusForUAT, StatusUATPassed, true},
		{StatusUATPassed, StatusForProduction, true},
		{StatusClosed, StatusInProgress, false},
	}
	for _, c := range cases {
		got := IsValidTransition(c.from, c.to)
		if got != c.want {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
