CREATE TABLE ticket_categories (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    parent_id   TEXT REFERENCES ticket_categories(id) ON DELETE SET NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE projects (
    id          TEXT PRIMARY KEY,
    project_key TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    status      TEXT NOT NULL DEFAULT 'PLANNING', -- PLANNING, ACTIVE, ON_HOLD, COMPLETED, CANCELLED
    start_date  DATE,
    target_date DATE,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE project_members (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, user_id)
);

CREATE TABLE milestones (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    due_date    DATE,
    status      TEXT NOT NULL DEFAULT 'NOT_STARTED', -- NOT_STARTED, IN_PROGRESS, COMPLETED, OVERDUE
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_milestones_project ON milestones(project_id);

CREATE TABLE sla_policies (
    id                TEXT PRIMARY KEY,
    severity          TEXT NOT NULL UNIQUE, -- S1, S2, S3, S4
    name              TEXT NOT NULL,
    response_minutes  INTEGER NOT NULL,     -- target minutes to first acknowledgement
    resolution_minutes INTEGER NOT NULL,    -- target minutes to resolution
    use_business_hours INTEGER NOT NULL DEFAULT 1, -- if 0, resolution counts wall-clock minutes
    is_active         INTEGER NOT NULL DEFAULT 1,
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE tickets (
    id                  TEXT PRIMARY KEY,
    ticket_number       TEXT NOT NULL UNIQUE, -- T2026-00001
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    requester_id        TEXT REFERENCES users(id) ON DELETE SET NULL,
    assignee_id         TEXT REFERENCES users(id) ON DELETE SET NULL,
    team_id             TEXT REFERENCES teams(id) ON DELETE SET NULL,
    project_id          TEXT REFERENCES projects(id) ON DELETE SET NULL,
    category_id         TEXT REFERENCES ticket_categories(id) ON DELETE SET NULL,
    severity            TEXT NOT NULL DEFAULT 'S3', -- S1, S2, S3, S4
    priority            TEXT NOT NULL DEFAULT 'Medium', -- Critical, High, Medium, Low
    status              TEXT NOT NULL DEFAULT 'NEW',
    environment         TEXT NOT NULL DEFAULT 'PRODUCTION', -- DEV, UAT, PRE-PROD, PRODUCTION
    sla_policy_id       TEXT REFERENCES sla_policies(id) ON DELETE SET NULL,

    response_deadline   DATETIME,
    resolution_deadline DATETIME,

    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    acknowledged_at     DATETIME,
    resolved_at         DATETIME,
    closed_at           DATETIME,
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    last_followup_at    DATETIME,
    next_followup_at    DATETIME,

    rca_required        INTEGER NOT NULL DEFAULT 0,
    escalation_level     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_tickets_status ON tickets(status);
CREATE INDEX idx_tickets_severity ON tickets(severity);
CREATE INDEX idx_tickets_assignee ON tickets(assignee_id);
CREATE INDEX idx_tickets_team ON tickets(team_id);
CREATE INDEX idx_tickets_project ON tickets(project_id);
CREATE INDEX idx_tickets_requester ON tickets(requester_id);
CREATE INDEX idx_tickets_created ON tickets(created_at);
CREATE INDEX idx_tickets_resolution_deadline ON tickets(resolution_deadline);

CREATE TABLE ticket_comments (
    id         TEXT PRIMARY KEY,
    ticket_id  TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    content    TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    is_deleted INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_comments_ticket ON ticket_comments(ticket_id);

CREATE TABLE ticket_history (
    id          TEXT PRIMARY KEY,
    ticket_id   TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    event_type  TEXT NOT NULL, -- CREATED, ASSIGNED, STATUS_CHANGED, SEVERITY_CHANGED, ...
    field       TEXT NOT NULL DEFAULT '',
    old_value   TEXT NOT NULL DEFAULT '',
    new_value   TEXT NOT NULL DEFAULT '',
    note        TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_history_ticket ON ticket_history(ticket_id);
CREATE INDEX idx_history_created ON ticket_history(created_at);

CREATE TABLE ticket_attachments (
    id           TEXT PRIMARY KEY,
    ticket_id    TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id      TEXT REFERENCES users(id) ON DELETE SET NULL,
    file_name    TEXT NOT NULL,
    stored_name  TEXT NOT NULL, -- randomized on-disk name to prevent traversal/collision
    content_type TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_attachments_ticket ON ticket_attachments(ticket_id);

CREATE TABLE ticket_watchers (
    ticket_id TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (ticket_id, user_id)
);

CREATE TABLE saved_views (
    id         TEXT PRIMARY KEY,
    user_id    TEXT REFERENCES users(id) ON DELETE CASCADE, -- NULL = system default view
    name       TEXT NOT NULL,
    filters    TEXT NOT NULL DEFAULT '{}', -- JSON
    is_system  INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Seed default SLA policies (spec section 28)
INSERT INTO sla_policies (id, severity, name, response_minutes, resolution_minutes, use_business_hours) VALUES
    ('sla-s1', 'S1', 'Critical', 15, 240, 0),
    ('sla-s2', 'S2', 'High', 30, 480, 0),
    ('sla-s3', 'S3', 'Medium', 120, 960, 1),   -- 2 business days = 2*8h*60
    ('sla-s4', 'S4', 'Low', 240, 2400, 1);     -- 5 business days = 5*8h*60

-- Seed default saved views (spec section 47)
INSERT INTO saved_views (id, user_id, name, filters, is_system) VALUES
    ('view-my-open', NULL, 'My Open Tickets', '{"assignee":"me","status_not_in":["RESOLVED","CLOSED","CANCELLED"]}', 1),
    ('view-s1-s2', NULL, 'S1/S2', '{"severity_in":["S1","S2"]}', 1),
    ('view-sla-breached', NULL, 'SLA Breached', '{"sla_status":"BREACHED"}', 1),
    ('view-sla-warning', NULL, 'SLA Warning', '{"sla_status":"WARNING"}', 1),
    ('view-followup-required', NULL, 'Follow-up Required', '{"followup_required":true}', 1),
    ('view-waiting-dev', NULL, 'Waiting for Development', '{"status_in":["IN_PROGRESS"]}', 1),
    ('view-waiting-qa', NULL, 'Waiting for QA', '{"status_in":["FOR_UAT"]}', 1),
    ('view-ready-uat', NULL, 'Ready for UAT', '{"status_in":["FOR_UAT"]}', 1),
    ('view-ready-prod', NULL, 'Ready for Production', '{"status_in":["FOR_PRODUCTION"]}', 1);
