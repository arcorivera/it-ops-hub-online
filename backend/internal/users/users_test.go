package users

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

func TestCreateAndGetUser(t *testing.T) {
	repo := newTestRepo(t)

	u, err := repo.Create(CreateInput{
		Username: "jdoe",
		FullName: "Jane Doe",
		Email:    "jane@example.com",
		Roles:    []string{RoleAgent, RoleTeamLead},
	}, "hashed-password")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected generated ID")
	}
	if len(u.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d: %v", len(u.Roles), u.Roles)
	}

	fetched, err := repo.GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.Username != "jdoe" {
		t.Errorf("expected username jdoe, got %s", fetched.Username)
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.Create(CreateInput{Username: "a", FullName: "A", Email: "dup@example.com"}, "h")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = repo.Create(CreateInput{Username: "b", FullName: "B", Email: "dup@example.com"}, "h")
	if err != ErrDuplicateEmail {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestCreateDuplicateUsername(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.Create(CreateInput{Username: "dupuser", FullName: "A", Email: "a@example.com"}, "h")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err = repo.Create(CreateInput{Username: "dupuser", FullName: "B", Email: "b@example.com"}, "h")
	if err != ErrDuplicateUsername {
		t.Errorf("expected ErrDuplicateUsername, got %v", err)
	}
}

func TestUpdateUser(t *testing.T) {
	repo := newTestRepo(t)
	u, _ := repo.Create(CreateInput{Username: "upd", FullName: "Old Name", Email: "upd@example.com", Roles: []string{RoleViewer}}, "h")

	newName := "New Name"
	inactive := false
	newRoles := []string{RoleAgent}

	updated, err := repo.Update(u.ID, UpdateInput{
		FullName: &newName,
		IsActive: &inactive,
		Roles:    &newRoles,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.FullName != "New Name" {
		t.Errorf("expected updated full name, got %s", updated.FullName)
	}
	if updated.IsActive {
		t.Error("expected user to be inactive")
	}
	if len(updated.Roles) != 1 || updated.Roles[0] != RoleAgent {
		t.Errorf("expected roles [AGENT], got %v", updated.Roles)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.GetByID("nope")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCount(t *testing.T) {
	repo := newTestRepo(t)
	n, err := repo.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 users initially, got %d", n)
	}
	repo.Create(CreateInput{Username: "x", FullName: "X", Email: "x@example.com"}, "h")
	n, _ = repo.Count()
	if n != 1 {
		t.Errorf("expected 1 user after create, got %d", n)
	}
}
