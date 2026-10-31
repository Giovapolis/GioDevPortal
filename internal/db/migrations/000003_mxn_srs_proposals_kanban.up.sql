ALTER TABLE projects ADD COLUMN kanban_srs_version_id INTEGER REFERENCES srs_versions(id) ON DELETE SET NULL;
UPDATE projects SET currency = 'MXN';

CREATE TABLE payment_claims_new (
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

INSERT INTO payment_claims_new (id, project_id, client_id, amount, note, proof_key, proof_name, proof_content_type, proof_size, status, admin_note, billing_id, reviewed_by, reviewed_at, created_at)
SELECT id, project_id, client_id, amount, note, proof_key, proof_name, proof_content_type, proof_size,
       CASE WHEN status = 'approved' THEN 'paid' ELSE 'pending' END,
       admin_note, billing_id, reviewed_by, reviewed_at, created_at
FROM payment_claims;

DROP TABLE payment_claims;
ALTER TABLE payment_claims_new RENAME TO payment_claims;
CREATE INDEX idx_payment_claims_project ON payment_claims(project_id, status);

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

CREATE INDEX idx_srs_proposals_project ON srs_proposals(project_id, status);
