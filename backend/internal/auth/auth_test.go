package auth

import (
	"testing"

	"itopshub/backend/internal/database"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !CheckPassword(hash, "correct-horse-battery-staple") {
		t.Error("expected correct password to validate")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("expected incorrect password to fail validation")
	}
}

func TestSessionLifecycle(t *testing.T) {
	db, err := database.Open(t.TempDir()+"/test.db", nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Need a real user row to satisfy the FK on sessions.user_id.
	_, err = db.Exec(`INSERT INTO users (id, username, full_name, email, password_hash) VALUES ('u1','tester','Tester','t@example.com','x')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	store := NewStore(db, 24)

	sess, err := store.CreateSession("u1", "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("expected non-empty session token")
	}

	got, err := store.ValidateSession(sess.ID)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if got.UserID != "u1" {
		t.Errorf("expected user u1, got %s", got.UserID)
	}

	if err := store.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := store.ValidateSession(sess.ID); err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestValidateSession_Unknown(t *testing.T) {
	db, err := database.Open(t.TempDir()+"/test.db", nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewStore(db, 24)
	if _, err := store.ValidateSession("does-not-exist"); err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}
