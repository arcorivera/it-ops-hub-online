-- sla_events: records pause/resume history and status transitions so the
-- SLA engine can compute effective elapsed time (stopwatch semantics).
CREATE TABLE sla_events (
    id         TEXT PRIMARY KEY,
    ticket_id  TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL, -- STARTED, PAUSED, RESUMED, WARNING, CRITICAL, BREACHED, RESOLVED
    note       TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_sla_events_ticket ON sla_events(ticket_id);
CREATE INDEX idx_sla_events_type ON sla_events(ticket_id, event_type);

CREATE TABLE follow_up_rules (
    id                TEXT PRIMARY KEY,
    severity          TEXT NOT NULL UNIQUE, -- S1..S4
    interval_minutes  INTEGER NOT NULL,     -- how often a follow-up should be raised while open
    is_active         INTEGER NOT NULL DEFAULT 1,
    updated_at        DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE follow_ups (
    id         TEXT PRIMARY KEY,
    ticket_id  TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL, -- NULL = system-generated
    reason     TEXT NOT NULL DEFAULT '',
    is_system  INTEGER NOT NULL DEFAULT 0,
    resolved   INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_followups_ticket ON follow_ups(ticket_id);

CREATE TABLE escalation_rules (
    id                TEXT PRIMARY KEY,
    severity          TEXT NOT NULL, -- S1..S4
    threshold_percent INTEGER NOT NULL, -- 50, 75, 100 (100 = breach)
    escalate_to_role  TEXT NOT NULL,    -- ASSIGNEE, TEAM_LEAD, IT_MANAGER
    is_active         INTEGER NOT NULL DEFAULT 1,
    UNIQUE(severity, threshold_percent)
);

CREATE TABLE escalation_events (
    id             TEXT PRIMARY KEY,
    ticket_id      TEXT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    rule_id        TEXT REFERENCES escalation_rules(id) ON DELETE SET NULL,
    threshold_percent INTEGER NOT NULL,
    escalated_to_role TEXT NOT NULL,
    escalated_to_user TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at     DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_escalation_events_ticket ON escalation_events(ticket_id);

CREATE TABLE notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL, -- NEW_TICKET, ASSIGNED, COMMENT, SLA_WARNING, SLA_CRITICAL, SLA_BREACHED, FOLLOW_UP, ESCALATION, UAT_FAILED, UAT_PASSED, PRE_PROD_FAILED, PRE_PROD_PASSED, PRODUCTION_DEPLOYMENT, RESOLVED
    title      TEXT NOT NULL,
    message    TEXT NOT NULL DEFAULT '',
    entity_type TEXT NOT NULL DEFAULT '', -- ticket, incident, project
    entity_id   TEXT NOT NULL DEFAULT '',
    is_read    INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_notifications_user ON notifications(user_id, is_read);
CREATE INDEX idx_notifications_created ON notifications(created_at);

-- Seed default follow-up rules (spec section 33)
INSERT INTO follow_up_rules (id, severity, interval_minutes) VALUES
    ('fu-s1', 'S1', 30),
    ('fu-s2', 'S2', 60),
    ('fu-s3', 'S3', 240),
    ('fu-s4', 'S4', 480); -- 1 business day approximated as 480 min (8h)

-- Seed default escalation rules (spec section 34)
INSERT INTO escalation_rules (id, severity, threshold_percent, escalate_to_role) VALUES
    ('esc-s1-50',  'S1', 50,  'ASSIGNEE'),
    ('esc-s1-75',  'S1', 75,  'TEAM_LEAD'),
    ('esc-s1-100', 'S1', 100, 'IT_MANAGER'),
    ('esc-s2-50',  'S2', 50,  'ASSIGNEE'),
    ('esc-s2-75',  'S2', 75,  'TEAM_LEAD'),
    ('esc-s2-100', 'S2', 100, 'IT_MANAGER'),
    ('esc-s3-75',  'S3', 75,  'TEAM_LEAD'),
    ('esc-s3-100', 'S3', 100, 'IT_MANAGER'),
    ('esc-s4-100', 'S4', 100, 'IT_MANAGER');
