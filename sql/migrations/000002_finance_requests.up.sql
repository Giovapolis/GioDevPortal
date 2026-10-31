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
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
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
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    admin_note TEXT NOT NULL DEFAULT '',
    srs_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL,
    reviewed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_payment_claims_project ON payment_claims(project_id, status);
CREATE INDEX idx_srs_requests_project ON srs_requests(project_id, status);
