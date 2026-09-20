// Package testing implements UAT test-case management (spec section 39-40)
// and Pre-Prod/Production deployment record tracking (spec sections 41-42).
package testing

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("test case not found")
var ErrDeploymentNotFound = errors.New("deployment not found")

// Test case statuses (spec section 39).
const (
	TestNotStarted = "NOT_STARTED"
	TestPass       = "PASS"
	TestFail       = "FAIL"
	TestBlocked    = "BLOCKED"
)

var ValidTestStatuses = map[string]bool{TestNotStarted: true, TestPass: true, TestFail: true, TestBlocked: true}

// Deployment stages and validation results (spec sections 41-42).
const (
	StagePreProd    = "PRE_PROD"
	StageProduction = "PRODUCTION"

	ValidationPending = "PENDING"
	ValidationPassed  = "PASSED"
	ValidationFailed  = "FAILED"
)

type TestCase struct {
	ID             string     `json:"id"`
	TicketID       string     `json:"ticketId"`
	Description    string     `json:"description"`
	Environment    string     `json:"environment"`
	Preconditions  string     `json:"preconditions"`
	ExpectedResult string     `json:"expectedResult"`
	ActualResult   string     `json:"actualResult"`
	TesterID       *string    `json:"testerId"`
	Status         string     `json:"status"`
	ExecutedAt     *time.Time `json:"executedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Deployment struct {
	ID                  string    `json:"id"`
	TicketID            string    `json:"ticketId"`
	Stage               string    `json:"stage"`
	Version             string    `json:"version"`
	DeploymentReference string    `json:"deploymentReference"`
	DeployedBy          *string   `json:"deployedBy"`
	DeployedAt          time.Time `json:"deployedAt"`
	ValidationResult    string    `json:"validationResult"`
	RollbackReason      string    `json:"rollbackReason"`
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- Test cases ---

type CreateTestCaseInput struct {
	TicketID       string
	Description    string
	Environment    string
	Preconditions  string
	ExpectedResult string
}

func (r *Repository) CreateTestCase(in CreateTestCaseInput) (*TestCase, error) {
	id := "tc-" + uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.Exec(
		`INSERT INTO test_cases (id, ticket_id, description, environment, preconditions, expected_result, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.TicketID, in.Description, in.Environment, in.Preconditions, in.ExpectedResult, TestNotStarted, now, now,
	)
	if err != nil {
		return nil, err
	}
	return r.GetTestCase(id)
}

func (r *Repository) GetTestCase(id string) (*TestCase, error) {
	row := r.db.QueryRow(`
		SELECT id, ticket_id, description, environment, preconditions, expected_result, actual_result, tester_id, status, executed_at, created_at, updated_at
		FROM test_cases WHERE id = ?`, id)
	return scanTestCase(row)
}

func scanTestCase(row *sql.Row) (*TestCase, error) {
	var tc TestCase
	var testerID, executedAt sql.NullString
	var actualResult sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&tc.ID, &tc.TicketID, &tc.Description, &tc.Environment, &tc.Preconditions, &tc.ExpectedResult,
		&actualResult, &testerID, &tc.Status, &executedAt, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	tc.ActualResult = actualResult.String
	if testerID.Valid {
		tc.TesterID = &testerID.String
	}
	if executedAt.Valid && executedAt.String != "" {
		t, _ := time.Parse(time.RFC3339, normalizeTime(executedAt.String))
		tc.ExecutedAt = &t
	}
	tc.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
	tc.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
	return &tc, nil
}

func (r *Repository) ListTestCases(ticketID string) ([]*TestCase, error) {
	rows, err := r.db.Query(`
		SELECT id, ticket_id, description, environment, preconditions, expected_result, actual_result, tester_id, status, executed_at, created_at, updated_at
		FROM test_cases WHERE ticket_id = ? ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*TestCase, 0)
	for rows.Next() {
		var tc TestCase
		var testerID, executedAt, actualResult sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&tc.ID, &tc.TicketID, &tc.Description, &tc.Environment, &tc.Preconditions, &tc.ExpectedResult,
			&actualResult, &testerID, &tc.Status, &executedAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		tc.ActualResult = actualResult.String
		if testerID.Valid {
			tc.TesterID = &testerID.String
		}
		if executedAt.Valid && executedAt.String != "" {
			t, _ := time.Parse(time.RFC3339, normalizeTime(executedAt.String))
			tc.ExecutedAt = &t
		}
		tc.CreatedAt, _ = time.Parse(time.RFC3339, normalizeTime(createdAt))
		tc.UpdatedAt, _ = time.Parse(time.RFC3339, normalizeTime(updatedAt))
		out = append(out, &tc)
	}
	return out, nil
}

// Execute records a test result (spec section 39: Execute/Pass/Fail/Block),
// stamping the tester, timestamp, and actual result, and logs a
// corresponding row in test_results for history.
func (r *Repository) Execute(testCaseID, testerID, status, actualResult, comment string) (*TestCase, error) {
	if !ValidTestStatuses[status] || status == TestNotStarted {
		return nil, errors.New("invalid execution status")
	}
	now := time.Now().UTC()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE test_cases SET status=?, actual_result=?, tester_id=?, executed_at=?, updated_at=? WHERE id=?`,
		status, actualResult, testerID, now, now, testCaseID,
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(
		`INSERT INTO test_results (id, test_case_id, user_id, status, comment, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"tr-"+uuid.NewString(), testCaseID, testerID, status, comment, now,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTestCase(testCaseID)
}

// HasFailingOrIncompleteTests implements the UAT gate (spec section 40):
// a ticket cannot move to UAT_PASSED while any of its test cases are failed,
// blocked, or not yet executed. Tickets with zero test cases are not gated
// (nothing was ever required).
func (r *Repository) HasFailingOrIncompleteTests(ticketID string) (bool, error) {
	var total, blocking int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM test_cases WHERE ticket_id = ?`, ticketID).Scan(&total); err != nil {
		return false, err
	}
	if total == 0 {
		return false, nil
	}
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM test_cases WHERE ticket_id = ? AND status IN ('NOT_STARTED','FAIL','BLOCKED')`, ticketID,
	).Scan(&blocking)
	if err != nil {
		return false, err
	}
	return blocking > 0, nil
}

// --- Deployments ---

type CreateDeploymentInput struct {
	TicketID            string
	Stage               string
	Version             string
	DeploymentReference string
	DeployedBy          string
	ValidationResult    string
}

func (r *Repository) CreateDeployment(in CreateDeploymentInput) (*Deployment, error) {
	id := "dep-" + uuid.NewString()
	now := time.Now().UTC()

	validation := in.ValidationResult
	if validation == "" {
		validation = ValidationPending
	}

	var deployedBy interface{}
	if in.DeployedBy != "" {
		deployedBy = in.DeployedBy
	}

	_, err := r.db.Exec(
		`INSERT INTO deployments (id, ticket_id, stage, version, deployment_reference, deployed_by, deployed_at, validation_result, rollback_reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')`,
		id, in.TicketID, in.Stage, in.Version, in.DeploymentReference, deployedBy, now, validation,
	)
	if err != nil {
		return nil, err
	}
	return r.GetDeployment(id)
}

func (r *Repository) GetDeployment(id string) (*Deployment, error) {
	row := r.db.QueryRow(`SELECT id, ticket_id, stage, version, deployment_reference, deployed_by, deployed_at, validation_result, rollback_reason FROM deployments WHERE id = ?`, id)
	var d Deployment
	var deployedBy sql.NullString
	var deployedAt string
	if err := row.Scan(&d.ID, &d.TicketID, &d.Stage, &d.Version, &d.DeploymentReference, &deployedBy, &deployedAt, &d.ValidationResult, &d.RollbackReason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDeploymentNotFound
		}
		return nil, err
	}
	if deployedBy.Valid {
		d.DeployedBy = &deployedBy.String
	}
	d.DeployedAt, _ = time.Parse(time.RFC3339, normalizeTime(deployedAt))
	return &d, nil
}

func (r *Repository) ListDeployments(ticketID string) ([]*Deployment, error) {
	rows, err := r.db.Query(`SELECT id, ticket_id, stage, version, deployment_reference, deployed_by, deployed_at, validation_result, rollback_reason FROM deployments WHERE ticket_id = ? ORDER BY deployed_at DESC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Deployment, 0)
	for rows.Next() {
		var d Deployment
		var deployedBy sql.NullString
		var deployedAt string
		if err := rows.Scan(&d.ID, &d.TicketID, &d.Stage, &d.Version, &d.DeploymentReference, &deployedBy, &deployedAt, &d.ValidationResult, &d.RollbackReason); err != nil {
			return nil, err
		}
		if deployedBy.Valid {
			d.DeployedBy = &deployedBy.String
		}
		d.DeployedAt, _ = time.Parse(time.RFC3339, normalizeTime(deployedAt))
		out = append(out, &d)
	}
	return out, nil
}

func (r *Repository) UpdateDeploymentValidation(id, result, rollbackReason string) (*Deployment, error) {
	_, err := r.db.Exec(`UPDATE deployments SET validation_result = ?, rollback_reason = ? WHERE id = ?`, result, rollbackReason, id)
	if err != nil {
		return nil, err
	}
	return r.GetDeployment(id)
}

// TicketInfoForNotify returns the minimal ticket fields needed to route a
// notification (assignee, number, title) without the testing package
// needing to import the tickets package (would create a cycle).
func (r *Repository) TicketInfoForNotify(ticketID string) (assigneeID, number, title string) {
	var a, n, t sql.NullString
	row := r.db.QueryRow(`SELECT assignee_id, ticket_number, title FROM tickets WHERE id = ?`, ticketID)
	if err := row.Scan(&a, &n, &t); err == nil {
		assigneeID, number, title = a.String, n.String, t.String
	}
	return
}

func normalizeTime(v string) string {
	if len(v) == 19 && v[10] == ' ' {
		return v[:10] + "T" + v[11:] + "Z"
	}
	return v
}
