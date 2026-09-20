CREATE TABLE test_cases (
    id               TEXT PRIMARY KEY,
    ticket_id        TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    description      TEXT NOT NULL,
    environment      TEXT NOT NULL DEFAULT 'UAT',
    preconditions    TEXT NOT NULL DEFAULT '',
    expected_result  TEXT NOT NULL DEFAULT '',
    actual_result    TEXT NOT NULL DEFAULT '',
    tester_id        TEXT REFERENCES users(id) ON DELETE SET NULL,
    status           TEXT NOT NULL DEFAULT 'NOT_STARTED', -- NOT_STARTED, PASS, FAIL, BLOCKED
    executed_at      DATETIME,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_test_cases_ticket ON test_cases(ticket_id);

CREATE TABLE test_results (
    id            TEXT PRIMARY KEY,
    test_case_id  TEXT NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
    user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    status        TEXT NOT NULL, -- PASS, FAIL, BLOCKED
    comment       TEXT NOT NULL DEFAULT '',
    evidence_attachment_id TEXT REFERENCES ticket_attachments(id) ON DELETE SET NULL,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_test_results_case ON test_results(test_case_id);

-- Deployment tracking for Pre-Prod / Production stages (spec sections 41-42)
CREATE TABLE deployments (
    id                  TEXT PRIMARY KEY,
    ticket_id           TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    stage               TEXT NOT NULL, -- PRE_PROD, PRODUCTION
    version             TEXT NOT NULL DEFAULT '',
    deployment_reference TEXT NOT NULL DEFAULT '',
    deployed_by         TEXT REFERENCES users(id) ON DELETE SET NULL,
    deployed_at         DATETIME NOT NULL DEFAULT (datetime('now')),
    validation_result   TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, PASSED, FAILED
    rollback_reason     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_deployments_ticket ON deployments(ticket_id);

CREATE TABLE incidents (
    id               TEXT PRIMARY KEY,
    incident_number  TEXT NOT NULL UNIQUE, -- INC2026-00001
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    severity         TEXT NOT NULL DEFAULT 'S3',
    impact           TEXT NOT NULL DEFAULT '',
    affected_system  TEXT NOT NULL DEFAULT '',
    environment      TEXT NOT NULL DEFAULT 'PRODUCTION',
    owner_id         TEXT REFERENCES users(id) ON DELETE SET NULL,
    status           TEXT NOT NULL DEFAULT 'OPEN', -- OPEN, INVESTIGATING, MITIGATED, RESOLVED, CLOSED
    ticket_id        TEXT REFERENCES tickets(id) ON DELETE SET NULL,
    started_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    resolved_at      DATETIME,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);

CREATE TABLE rca_records (
    id                     TEXT PRIMARY KEY,
    incident_id            TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    incident_summary       TEXT NOT NULL DEFAULT '',
    business_impact        TEXT NOT NULL DEFAULT '',
    root_cause             TEXT NOT NULL DEFAULT '',
    contributing_factors   TEXT NOT NULL DEFAULT '',
    timeline                TEXT NOT NULL DEFAULT '',
    immediate_fix          TEXT NOT NULL DEFAULT '',
    permanent_fix           TEXT NOT NULL DEFAULT '',
    preventive_action       TEXT NOT NULL DEFAULT '',
    responsible_team_id     TEXT REFERENCES teams(id) ON DELETE SET NULL,
    responsible_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    deployment_reference    TEXT NOT NULL DEFAULT '',
    status                  TEXT NOT NULL DEFAULT 'DRAFT', -- DRAFT, IN_REVIEW, COMPLETED
    override_by             TEXT REFERENCES users(id) ON DELETE SET NULL,
    override_reason         TEXT NOT NULL DEFAULT '',
    created_at              DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at              DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_rca_incident ON rca_records(incident_id);

CREATE TABLE backups (
    id          TEXT PRIMARY KEY,
    file_name   TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL,
    created_by  TEXT REFERENCES users(id) ON DELETE SET NULL,
    is_safety   INTEGER NOT NULL DEFAULT 0, -- auto-created before a restore
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
