DROP INDEX IF EXISTS idx_srs_proposals_project;
DROP TABLE IF EXISTS srs_proposals;

CREATE TABLE payment_claims_old (
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

INSERT INTO payment_claims_old (id, project_id, client_id, amount, note, proof_key, proof_name, proof_content_type, proof_size, status, admin_note, billing_id, reviewed_by, reviewed_at, created_at)
SELECT id, project_id, client_id, amount, note, proof_key, proof_name, proof_content_type, proof_size,
       CASE WHEN status = 'paid' THEN 'approved' ELSE 'pending' END,
       admin_note, billing_id, reviewed_by, reviewed_at, created_at
FROM payment_claims;

DROP TABLE payment_claims;
ALTER TABLE payment_claims_old RENAME TO payment_claims;
CREATE INDEX idx_payment_claims_project ON payment_claims(project_id, status);

ALTER TABLE projects DROP COLUMN kanban_srs_version_id;
