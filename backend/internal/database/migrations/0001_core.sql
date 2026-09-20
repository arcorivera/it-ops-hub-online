-- Core identity, RBAC, teams, sessions, settings, audit log

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    full_name     TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE roles (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE, -- ADMIN, IT_MANAGER, TEAM_LEAD, AGENT, REQUESTER, VIEWER
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_roles (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY, -- opaque random token, hashed
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    expires_at DATETIME NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE teams (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE team_members (
    team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_lead INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE audit_logs (
    id         TEXT PRIMARY KEY,
    user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    action     TEXT NOT NULL,        -- LOGIN, LOGOUT, CREATE, UPDATE, DELETE, ASSIGN, STATUS_CHANGE, ...
    entity_type TEXT NOT NULL,       -- ticket, user, sla_policy, ...
    entity_id  TEXT NOT NULL DEFAULT '',
    details    TEXT NOT NULL DEFAULT '{}', -- JSON blob, never contains passwords
    ip_address TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_audit_created ON audit_logs(created_at);
CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_user ON audit_logs(user_id);

-- Seed the fixed role catalog (spec section 10). Roles are not user-editable
-- as new roles, only assignable, to keep permission logic predictable.
INSERT INTO roles (id, name, description) VALUES
    ('role-admin',      'ADMIN',      'Full system administration'),
    ('role-it-manager', 'IT_MANAGER', 'Manage operational tickets and reports'),
    ('role-team-lead',  'TEAM_LEAD',  'Manage team tickets'),
    ('role-agent',      'AGENT',      'Manage assigned tickets'),
    ('role-requester',  'REQUESTER',  'Create tickets and view permitted tickets'),
    ('role-viewer',     'VIEWER',     'Read-only access');
