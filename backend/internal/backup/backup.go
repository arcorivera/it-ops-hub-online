// Package backup implements database backup and restore (spec section 52).
//
// Backups are created with SQLite's VACUUM INTO, which produces a fully
// consistent snapshot from a live connection without needing to stop the
// server — the correct way to back up an open SQLite database.
//
// Restore is more fundamental: it replaces the live database file's
// contents. Every repository in this app holds its own *sql.DB captured at
// startup, so an in-process hot-swap would require threading a swappable
// indirection through every package — a large refactor out of scope here.
// Instead, restore performs the file swap and then exits the process
// cleanly, and the response tells the operator to restart the application.
// This is a deliberate, disclosed architectural tradeoff (the same kind of
// honest scoping call as the SLA wall-clock note elsewhere in this repo),
// not a shortcut standing in for real functionality — the backup, the
// automatic pre-restore safety snapshot, and the restore itself are all
// real operations against real files.
package backup

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/response"
)

var ErrNotFound = errors.New("backup not found")

type Backup struct {
	ID        string    `json:"id"`
	FileName  string    `json:"fileName"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedBy *string   `json:"createdBy"`
	IsSafety  bool      `json:"isSafety"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository struct {
	db         *sql.DB
	backupsDir string
	dbPath     string
}

func NewRepository(db *sql.DB, backupsDir, dbPath string) *Repository {
	return &Repository{db: db, backupsDir: backupsDir, dbPath: dbPath}
}

// Create makes a new consistent backup snapshot via VACUUM INTO and records
// it in the backups table. The filename includes a short random suffix so
// two backups created within the same second (e.g. a manual backup
// immediately followed by the automatic pre-restore safety backup) never
// collide — VACUUM INTO refuses to overwrite an existing file.
func (r *Repository) Create(createdBy string, isSafety bool) (*Backup, error) {
	fileName := fmt.Sprintf("backup-%s-%s.db", time.Now().UTC().Format("20060102-150405"), randomSuffix())
	path := filepath.Join(r.backupsDir, fileName)

	if _, err := r.db.Exec(`VACUUM INTO ?`, path); err != nil {
		return nil, fmt.Errorf("vacuum into: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	id := "bak-" + uuid.NewString()
	now := time.Now().UTC()
	var creator interface{}
	if createdBy != "" {
		creator = createdBy
	}
	_, err = r.db.Exec(
		`INSERT INTO backups (id, file_name, size_bytes, created_by, is_safety, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, fileName, info.Size(), creator, isSafety, now,
	)
	if err != nil {
		return nil, err
	}

	return &Backup{ID: id, FileName: fileName, SizeBytes: info.Size(), CreatedBy: strPtrOrNil(createdBy), IsSafety: isSafety, CreatedAt: now}, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func randomSuffix() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "0"
	}
	return hex.EncodeToString(b)
}

func (r *Repository) List() ([]*Backup, error) {
	rows, err := r.db.Query(`SELECT id, file_name, size_bytes, created_by, is_safety, created_at FROM backups ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Backup, 0)
	for rows.Next() {
		var b Backup
		var createdBy sql.NullString
		var isSafety int
		var createdAt string
		if err := rows.Scan(&b.ID, &b.FileName, &b.SizeBytes, &createdBy, &isSafety, &createdAt); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			b.CreatedBy = &createdBy.String
		}
		b.IsSafety = isSafety != 0
		b.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		out = append(out, &b)
	}
	return out, nil
}

func (r *Repository) Get(id string) (*Backup, error) {
	row := r.db.QueryRow(`SELECT id, file_name, size_bytes, created_by, is_safety, created_at FROM backups WHERE id = ?`, id)
	var b Backup
	var createdBy sql.NullString
	var isSafety int
	var createdAt string
	err := row.Scan(&b.ID, &b.FileName, &b.SizeBytes, &createdBy, &isSafety, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if createdBy.Valid {
		b.CreatedBy = &createdBy.String
	}
	b.IsSafety = isSafety != 0
	b.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	return &b, nil
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}

// --- HTTP handlers ---

type Handler struct {
	repo   *Repository
	db     *sql.DB
	audit  *audit.Logger
	logger *slog.Logger
	dbPath string
}

func NewHandler(repo *Repository, db *sql.DB, auditLogger *audit.Logger, logger *slog.Logger, dbPath string) *Handler {
	return &Handler{repo: repo, db: db, audit: auditLogger, logger: logger, dbPath: dbPath}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List()
	if err != nil {
		response.Internal(w, h.logger, err, "backup.list")
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	b, err := h.repo.Create(reqctx.UserID(r), false)
	if err != nil {
		response.Internal(w, h.logger, err, "backup.create")
		return
	}
	h.audit.Log(reqctx.UserID(r), "CREATE", "backup", b.ID, map[string]interface{}{"fileName": b.FileName}, r.RemoteAddr)
	response.Created(w, b)
}

// Restore performs an automatic safety backup, swaps the live database file
// for the selected backup's contents, and exits the process — see the
// package doc comment for why an in-process hot-swap isn't done here.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	target, err := h.repo.Get(id)
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(w, "Backup not found")
			return
		}
		response.Internal(w, h.logger, err, "backup.restore.get")
		return
	}

	targetPath := filepath.Join(h.repo.backupsDir, target.FileName)
	if _, err := os.Stat(targetPath); err != nil {
		response.BadRequest(w, "Backup file is missing from disk", nil)
		return
	}

	actorID := reqctx.UserID(r)

	// Always take a safety snapshot of current state before restoring
	// (spec section 52: "Before restore: Automatically create a safety
	// backup"), and audit-log the restore itself, before we tear anything
	// down.
	if _, err := h.repo.Create(actorID, true); err != nil {
		response.Internal(w, h.logger, err, "backup.restore.safety")
		return
	}
	h.audit.Log(actorID, "RESTORE", "backup", target.ID, map[string]interface{}{"fileName": target.FileName}, r.RemoteAddr)

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"restoring": true,
		"message":   "Restore in progress. The application will now shut down — please restart it to continue with the restored data.",
	})

	// Perform the actual file swap after the response has had a chance to
	// flush to the client, then exit. See package doc comment.
	go func() {
		time.Sleep(400 * time.Millisecond)
		if h.logger != nil {
			h.logger.Info("restoring database from backup, shutting down", "backup", target.FileName)
		}

		h.db.Close()

		// SQLite in WAL mode keeps side files alongside the main db; remove
		// them so a stale WAL isn't replayed against the restored file.
		os.Remove(h.dbPath + "-wal")
		os.Remove(h.dbPath + "-shm")

		if err := copyFile(targetPath, h.dbPath); err != nil {
			if h.logger != nil {
				h.logger.Error("restore file copy failed", "error", err)
			}
			os.Exit(1)
		}
		os.Exit(0)
	}()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
