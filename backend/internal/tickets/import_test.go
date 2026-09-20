package tickets

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"itopshub/backend/internal/audit"
	"itopshub/backend/internal/database"
	"itopshub/backend/internal/notifications"
	"itopshub/backend/internal/reqctx"
	"itopshub/backend/internal/users"
)

func newImportTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := database.Open(t.TempDir()+"/test.db", nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	db.Exec(`INSERT INTO users (id, username, full_name, email, password_hash) VALUES ('actor1','a','a','a@example.com','x')`)

	repo := NewRepository(db)
	userRepo := users.NewRepository(db)
	notifRepo := notifications.NewRepository(db)
	auditLogger := audit.NewLogger(db)

	return NewHandler(repo, auditLogger, nil, t.TempDir(), userRepo, notifRepo)
}

func buildMultipartCSV(t *testing.T, csvContent string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "import.csv")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	part.Write([]byte(csvContent))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/import", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req = req.WithContext(reqctx.WithUserID(req.Context(), "actor1"))

	return req, httptest.NewRecorder()
}

func TestImportCSV_ValidRowsImported(t *testing.T) {
	h := newImportTestHandler(t)
	csvContent := "title,severity,priority,environment\n" +
		"First ticket,S1,Critical,PRODUCTION\n" +
		"Second ticket,S3,Medium,DEV\n"

	req, rec := buildMultipartCSV(t, csvContent)
	h.ImportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"imported":2`) {
		t.Errorf("expected 2 imported tickets, got body: %s", rec.Body.String())
	}
}

func TestImportCSV_MissingTitleSkipped(t *testing.T) {
	h := newImportTestHandler(t)
	csvContent := "title,severity\n" +
		",S1\n" +
		"Valid ticket,S2\n"

	req, rec := buildMultipartCSV(t, csvContent)
	h.ImportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"imported":1`) {
		t.Errorf("expected 1 imported, got: %s", body)
	}
	if !strings.Contains(body, `"skipped":1`) {
		t.Errorf("expected 1 skipped for missing title, got: %s", body)
	}
}

func TestImportCSV_InvalidSeverityFailsRow(t *testing.T) {
	h := newImportTestHandler(t)
	csvContent := "title,severity\n" +
		"Bad severity ticket,S9\n"

	req, rec := buildMultipartCSV(t, csvContent)
	h.ImportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"imported":0`) {
		t.Errorf("expected 0 imported for invalid severity, got: %s", body)
	}
	if !strings.Contains(body, "Invalid severity") {
		t.Errorf("expected error message about invalid severity, got: %s", body)
	}
}

func TestImportCSV_MissingTitleColumnRejected(t *testing.T) {
	h := newImportTestHandler(t)
	csvContent := "description,severity\nsomething,S1\n"

	req, rec := buildMultipartCSV(t, csvContent)
	h.ImportCSV(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when 'title' column is missing entirely, got %d", rec.Code)
	}
}

func TestImportCSV_DefaultsAppliedWhenColumnsOmitted(t *testing.T) {
	h := newImportTestHandler(t)
	csvContent := "title\nMinimal ticket\n"

	req, rec := buildMultipartCSV(t, csvContent)
	h.ImportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"imported":1`) {
		t.Errorf("expected a minimal single-column row to import successfully with defaults, got: %s", rec.Body.String())
	}
}

func TestImportCSV_ShortRowFewerColumnsThanHeader(t *testing.T) {
	h := newImportTestHandler(t)
	// Header declares 5 columns but this data row only supplies 1 —
	// should still import using defaults for the missing trailing columns.
	csvContent := "title,description,severity,priority,environment\nJust a title\n"

	req, rec := buildMultipartCSV(t, csvContent)
	h.ImportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"imported":1`) {
		t.Errorf("expected short row (fewer columns than header) to still import, got: %s", rec.Body.String())
	}
}
