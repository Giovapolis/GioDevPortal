-- GioDevPortal v0.1 schema (SQLite)
PRAGMA foreign_keys = ON;

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'client')),
    is_active INTEGER NOT NULL DEFAULT 1,
    must_change_password INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    client_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'completed', 'archived')),
    start_date TEXT,
    base_cost REAL NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'MXN',
    git_repo TEXT NOT NULL DEFAULT '',
    private_notes TEXT NOT NULL DEFAULT '',
    api_keys TEXT NOT NULL DEFAULT '',
    server_access TEXT NOT NULL DEFAULT '',
    staging_url TEXT NOT NULL DEFAULT '',
    kanban_srs_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE srs_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 1,
    client_content TEXT NOT NULL DEFAULT '',
    dev_content TEXT NOT NULL DEFAULT '',
    source_notes TEXT NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TEXT NOT NULL
);

CREATE TABLE proposed_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    srs_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL,
    ai_title TEXT NOT NULL,
    ai_description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid')),
    created_at TEXT NOT NULL
);

CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'backlog' CHECK (status IN ('backlog', 'doing', 'testing', 'done')),
    column_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE deliverables (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sprint_num INTEGER NOT NULL DEFAULT 1,
    title TEXT NOT NULL DEFAULT '',
    url_demo TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    is_accepted INTEGER NOT NULL DEFAULT 0,
    accepted_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE tickets_notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    deliverable_id INTEGER REFERENCES deliverables(id) ON DELETE SET NULL,
    client_id INTEGER NOT NULL REFERENCES users(id),
    client_message TEXT NOT NULL,
    admin_reply TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'in_progress', 'closed')),
    created_at TEXT NOT NULL
);

CREATE TABLE resources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    uploaded_by INTEGER NOT NULL REFERENCES users(id),
    file_name TEXT NOT NULL,
    r2_key TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE billings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    amount REAL NOT NULL DEFAULT 0,
    is_extra INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    paid_status TEXT NOT NULL DEFAULT 'pending' CHECK (paid_status IN ('pending', 'paid')),
    paid_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE password_resets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    used INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    revoked INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE project_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    actor_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_projects_client ON projects(client_id);
CREATE INDEX idx_srs_project ON srs_versions(project_id);
CREATE INDEX idx_proposed_tasks_project ON proposed_tasks(project_id, status);
CREATE INDEX idx_tasks_project_status ON tasks(project_id, status, column_order);
CREATE INDEX idx_deliverables_project ON deliverables(project_id);
CREATE INDEX idx_tickets_project ON tickets_notes(project_id);
CREATE INDEX idx_resources_project ON resources(project_id);
CREATE INDEX idx_billings_project ON billings(project_id);
CREATE INDEX idx_events_project ON project_events(project_id);
CREATE UNIQUE INDEX idx_users_email ON users(email);

CREATE TABLE payment_claims (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount REAL NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    proof_key TEXT NOT NULL DEFAULT '',
    proof_name TEXT NOT NULL DEFAULT '',
    proof_content_type TEXT NOT NULL DEFAULT '',
    proof_size INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid')),
    admin_note TEXT NOT NULL DEFAULT '',
    billing_id INTEGER REFERENCES billings(id) ON DELETE SET NULL,
    reviewed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE srs_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    client_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid')),
    admin_note TEXT NOT NULL DEFAULT '',
    srs_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL,
    reviewed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE srs_proposals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    base_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL,
    client_content TEXT NOT NULL,
    dev_content TEXT NOT NULL,
    source_notes TEXT NOT NULL DEFAULT '',
    admin_approved INTEGER NOT NULL DEFAULT 0,
    client_approved INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'applied', 'superseded')),
    applied_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL,
    created_by INTEGER NOT NULL REFERENCES users(id),
    admin_approved_at TEXT,
    client_approved_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_payment_claims_project ON payment_claims(project_id, status);
CREATE INDEX idx_srs_requests_project ON srs_requests(project_id, status);
CREATE INDEX idx_srs_proposals_project ON srs_proposals(project_id, status);
