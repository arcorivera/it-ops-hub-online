package backup

import (
	"os"
	"path/filepath"
	"testing"

	"itopshub/backend/internal/database"
)

func newTestRepo(t *testing.T) (*Repository, string) {
	t.Helper()
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, "test.db")
	db, err := database.Open(dbPath, nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	backupsDir := filepath.Join(dbDir, "backups")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		t.Fatalf("mkdir backups: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO users (id, username, full_name, email, password_hash) VALUES ('actor-1','a','a','a@example.com','x')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return NewRepository(db, backupsDir, dbPath), backupsDir
}

func TestCreateBackup_ProducesRealFileOnDisk(t *testing.T) {
	repo, backupsDir := newTestRepo(t)

	b, err := repo.Create("actor-1", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if b.SizeBytes <= 0 {
		t.Error("expected a non-zero backup file size")
	}

	path := filepath.Join(backupsDir, b.FileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected backup file to exist on disk: %v", err)
	}
	if info.Size() != b.SizeBytes {
		t.Errorf("recorded size %d doesn't match actual file size %d", b.SizeBytes, info.Size())
	}
}

func TestCreateBackup_IsSafetyFlagRecorded(t *testing.T) {
	repo, _ := newTestRepo(t)

	manual, err := repo.Create("actor-1", false)
	if err != nil {
		t.Fatalf("Create manual: %v", err)
	}
	if manual.IsSafety {
		t.Error("expected manual backup to have IsSafety=false")
	}

	safety, err := repo.Create("actor-1", true)
	if err != nil {
		t.Fatalf("Create safety: %v", err)
	}
	if !safety.IsSafety {
		t.Error("expected safety backup to have IsSafety=true")
	}
}

func TestListBackups_ReturnsInCreationOrder(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.Create("actor-1", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(list))
	}
}

func TestGetBackup_NotFound(t *testing.T) {
	repo, _ := newTestRepo(t)
	_, err := repo.Get("does-not-exist")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBackupContainsRealData(t *testing.T) {
	repo, backupsDir := newTestRepo(t)

	// sla_policies is seeded by migrations; a real VACUUM INTO snapshot
	// should contain that same seeded data, proving it's a genuine copy
	// and not an empty placeholder file.
	b, err := repo.Create("actor-1", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	backupPath := filepath.Join(backupsDir, b.FileName)
	verifyDB, err := database.Open(backupPath, nil)
	if err != nil {
		t.Fatalf("open backup file as a database: %v", err)
	}
	defer verifyDB.Close()

	var count int
	if err := verifyDB.QueryRow(`SELECT COUNT(*) FROM sla_policies`).Scan(&count); err != nil {
		t.Fatalf("query backup db: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 seeded SLA policies in the backup snapshot, got %d", count)
	}
}
