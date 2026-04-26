CREATE TABLE IF NOT EXISTS certificates (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	primary_domain TEXT NOT NULL,
	domains_json TEXT NOT NULL,
	email TEXT NOT NULL,
	snis_json TEXT NOT NULL,
	cert_path TEXT NOT NULL DEFAULT '',
	key_path TEXT NOT NULL DEFAULT '',
	expires_at TEXT,
	auto_renew INTEGER NOT NULL DEFAULT 1,
	renew_before_days INTEGER NOT NULL DEFAULT 30,
	status TEXT NOT NULL DEFAULT 'pending',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	CHECK (auto_renew IN (0, 1)),
	CHECK (renew_before_days > 0),
	CHECK (status IN ('pending', 'active', 'warning', 'expired', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_certificates_status ON certificates(status);
CREATE INDEX IF NOT EXISTS idx_certificates_expires_at ON certificates(expires_at);
CREATE INDEX IF NOT EXISTS idx_certificates_primary_domain ON certificates(primary_domain);

CREATE TABLE IF NOT EXISTS kong_targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	environment TEXT NOT NULL,
	admin_url TEXT NOT NULL,
	auth_type TEXT NOT NULL DEFAULT 'none',
	auth_header_name TEXT NOT NULL DEFAULT '',
	auth_header_value TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'unknown',
	last_checked_at TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	CHECK (auth_type IN ('none', 'custom-header')),
	CHECK (status IN ('online', 'offline', 'unknown'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_kong_targets_name ON kong_targets(name);
CREATE INDEX IF NOT EXISTS idx_kong_targets_status ON kong_targets(status);
CREATE INDEX IF NOT EXISTS idx_kong_targets_environment ON kong_targets(environment);

CREATE TABLE IF NOT EXISTS certificate_kong_targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	certificate_id INTEGER NOT NULL,
	kong_target_id INTEGER NOT NULL,
	kong_certificate_id TEXT NOT NULL DEFAULT '',
	sync_status TEXT NOT NULL DEFAULT 'not_synced',
	last_synced_at TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (certificate_id) REFERENCES certificates(id) ON DELETE CASCADE,
	FOREIGN KEY (kong_target_id) REFERENCES kong_targets(id) ON DELETE CASCADE,
	UNIQUE (certificate_id, kong_target_id),
	CHECK (sync_status IN ('pending', 'synced', 'failed', 'not_synced'))
);

CREATE INDEX IF NOT EXISTS idx_certificate_kong_targets_certificate_id ON certificate_kong_targets(certificate_id);
CREATE INDEX IF NOT EXISTS idx_certificate_kong_targets_kong_target_id ON certificate_kong_targets(kong_target_id);
CREATE INDEX IF NOT EXISTS idx_certificate_kong_targets_sync_status ON certificate_kong_targets(sync_status);

CREATE TABLE IF NOT EXISTS jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	certificate_id INTEGER,
	kong_target_id INTEGER,
	type TEXT NOT NULL,
	status TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	log TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL DEFAULT (datetime('now')),
	finished_at TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	FOREIGN KEY (certificate_id) REFERENCES certificates(id) ON DELETE SET NULL,
	FOREIGN KEY (kong_target_id) REFERENCES kong_targets(id) ON DELETE SET NULL,
	CHECK (type IN ('issue', 'renew', 'sync', 'test_kong', 'delete')),
	CHECK (status IN ('running', 'success', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_jobs_certificate_id ON jobs(certificate_id);
CREATE INDEX IF NOT EXISTS idx_jobs_kong_target_id ON jobs(kong_target_id);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_started_at ON jobs(started_at);
