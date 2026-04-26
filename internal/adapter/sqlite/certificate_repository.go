package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"kong-cert-lite/internal/domain"
)

type CertificateRepository struct {
	db *sql.DB
}

func NewCertificateRepository(db *sql.DB) *CertificateRepository {
	return &CertificateRepository{db: db}
}

func (r *CertificateRepository) List(ctx context.Context) ([]domain.Certificate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, primary_domain, domains_json, email, snis_json,
		       cert_path, key_path, expires_at, auto_renew, renew_before_days,
		       status, created_at, updated_at
		FROM certificates
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list certificates: %w", err)
	}
	defer rows.Close()

	var certificates []domain.Certificate
	for rows.Next() {
		certificate, err := scanCertificate(rows)
		if err != nil {
			return nil, err
		}
		certificates = append(certificates, certificate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate certificates: %w", err)
	}

	return certificates, nil
}

func (r *CertificateRepository) Get(ctx context.Context, id int64) (domain.Certificate, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, primary_domain, domains_json, email, snis_json,
		       cert_path, key_path, expires_at, auto_renew, renew_before_days,
		       status, created_at, updated_at
		FROM certificates
		WHERE id = ?
	`, id)

	certificate, err := scanCertificate(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Certificate{}, domain.ErrNotFound
		}
		return domain.Certificate{}, fmt.Errorf("get certificate: %w", err)
	}

	return certificate, nil
}

func (r *CertificateRepository) Create(ctx context.Context, certificate domain.Certificate) (int64, error) {
	domainsJSON, err := json.Marshal(certificate.Domains)
	if err != nil {
		return 0, fmt.Errorf("marshal domains: %w", err)
	}
	snisJSON, err := json.Marshal(certificate.SNIs)
	if err != nil {
		return 0, fmt.Errorf("marshal snis: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO certificates (
			name, primary_domain, domains_json, email, snis_json,
			auto_renew, renew_before_days, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		certificate.Name,
		certificate.PrimaryDomain,
		string(domainsJSON),
		certificate.Email,
		string(snisJSON),
		boolToInt(certificate.AutoRenew),
		certificate.RenewBeforeDays,
		string(certificate.Status),
	)
	if err != nil {
		return 0, fmt.Errorf("create certificate: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read certificate id: %w", err)
	}

	return id, nil
}

func (r *CertificateRepository) Update(ctx context.Context, certificate domain.Certificate) error {
	domainsJSON, err := json.Marshal(certificate.Domains)
	if err != nil {
		return fmt.Errorf("marshal domains: %w", err)
	}
	snisJSON, err := json.Marshal(certificate.SNIs)
	if err != nil {
		return fmt.Errorf("marshal snis: %w", err)
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE certificates
		SET name = ?,
		    primary_domain = ?,
		    domains_json = ?,
		    email = ?,
		    snis_json = ?,
		    auto_renew = ?,
		    renew_before_days = ?,
		    updated_at = datetime('now')
		WHERE id = ?
	`,
		certificate.Name,
		certificate.PrimaryDomain,
		string(domainsJSON),
		certificate.Email,
		string(snisJSON),
		boolToInt(certificate.AutoRenew),
		certificate.RenewBeforeDays,
		certificate.ID,
	)
	if err != nil {
		return fmt.Errorf("update certificate: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update certificate rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *CertificateRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM certificates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete certificate: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete certificate rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *CertificateRepository) MarkIssued(ctx context.Context, id int64, certPath string, keyPath string, expiresAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE certificates
		SET cert_path = ?,
		    key_path = ?,
		    expires_at = ?,
		    status = ?,
		    updated_at = datetime('now')
		WHERE id = ?
	`, certPath, keyPath, expiresAt.UTC().Format("2006-01-02 15:04:05"), string(domain.CertificateStatusActive), id)
	if err != nil {
		return fmt.Errorf("mark issued certificate: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read mark issued rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *CertificateRepository) UpdateStatus(ctx context.Context, id int64, status domain.CertificateStatus) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE certificates
		SET status = ?,
		    updated_at = datetime('now')
		WHERE id = ?
	`, string(status), id)
	if err != nil {
		return fmt.Errorf("update certificate status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update status rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *CertificateRepository) ListKongTargets(ctx context.Context) ([]domain.KongTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, environment, admin_url, auth_type, auth_header_name,
		       auth_header_value, status, last_checked_at, created_at, updated_at
		FROM kong_targets
		ORDER BY environment ASC, name ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list kong targets for certificate: %w", err)
	}
	defer rows.Close()

	var targets []domain.KongTarget
	for rows.Next() {
		target, err := scanKongTarget(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kong targets for certificate: %w", err)
	}

	return targets, nil
}

func (r *CertificateRepository) ListSyncTargets(ctx context.Context, certificateID int64) ([]domain.CertificateKongTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, certificate_id, kong_target_id, kong_certificate_id,
		       sync_status, last_synced_at, last_error, created_at, updated_at
		FROM certificate_kong_targets
		WHERE certificate_id = ?
		ORDER BY kong_target_id ASC, id ASC
	`, certificateID)
	if err != nil {
		return nil, fmt.Errorf("list certificate kong targets: %w", err)
	}
	defer rows.Close()

	var mappings []domain.CertificateKongTarget
	for rows.Next() {
		mapping, err := scanCertificateKongTarget(rows)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate certificate kong targets: %w", err)
	}

	return mappings, nil
}

func (r *CertificateRepository) SetLinkedTargets(ctx context.Context, certificateID int64, targetIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin set linked targets: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if len(targetIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM certificate_kong_targets WHERE certificate_id = ?`, certificateID); err != nil {
			return fmt.Errorf("clear linked targets: %w", err)
		}
	} else {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(targetIDs)), ",")
		args := make([]any, 0, len(targetIDs)+1)
		args = append(args, certificateID)
		for _, targetID := range targetIDs {
			args = append(args, targetID)
		}
		deleteQuery := fmt.Sprintf(`DELETE FROM certificate_kong_targets WHERE certificate_id = ? AND kong_target_id NOT IN (%s)`, placeholders)
		if _, err := tx.ExecContext(ctx, deleteQuery, args...); err != nil {
			return fmt.Errorf("delete unlinked kong targets: %w", err)
		}
		for _, targetID := range targetIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO certificate_kong_targets (
					certificate_id, kong_target_id, sync_status, last_error
				) VALUES (?, ?, ?, '')
			`, certificateID, targetID, string(domain.SyncStatusNotSynced)); err != nil {
				return fmt.Errorf("insert linked kong target: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit linked targets: %w", err)
	}
	committed = true
	return nil
}

func (r *CertificateRepository) GetSyncTarget(ctx context.Context, certificateID int64, kongTargetID int64) (domain.CertificateKongTarget, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, certificate_id, kong_target_id, kong_certificate_id,
		       sync_status, last_synced_at, last_error, created_at, updated_at
		FROM certificate_kong_targets
		WHERE certificate_id = ? AND kong_target_id = ?
	`, certificateID, kongTargetID)

	mapping, err := scanCertificateKongTarget(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CertificateKongTarget{}, domain.ErrNotFound
		}
		return domain.CertificateKongTarget{}, fmt.Errorf("get certificate kong target: %w", err)
	}

	return mapping, nil
}

func (r *CertificateRepository) UpsertSyncTarget(ctx context.Context, mapping domain.CertificateKongTarget) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO certificate_kong_targets (
			certificate_id, kong_target_id, kong_certificate_id,
			sync_status, last_synced_at, last_error
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(certificate_id, kong_target_id) DO UPDATE SET
			kong_certificate_id = excluded.kong_certificate_id,
			sync_status = excluded.sync_status,
			last_synced_at = excluded.last_synced_at,
			last_error = excluded.last_error,
			updated_at = datetime('now')
	`,
		mapping.CertificateID,
		mapping.KongTargetID,
		mapping.KongCertificateID,
		string(mapping.SyncStatus),
		nullableTime(mapping.LastSyncedAt),
		mapping.LastError,
	)
	if err != nil {
		return fmt.Errorf("upsert certificate kong target: %w", err)
	}

	return nil
}

func scanCertificate(scanner interface {
	Scan(dest ...any) error
}) (domain.Certificate, error) {
	var certificate domain.Certificate
	var domainsJSON string
	var snisJSON string
	var expiresAt sql.NullString
	var createdAt string
	var updatedAt string
	var autoRenew int
	var status string

	err := scanner.Scan(
		&certificate.ID,
		&certificate.Name,
		&certificate.PrimaryDomain,
		&domainsJSON,
		&certificate.Email,
		&snisJSON,
		&certificate.CertPath,
		&certificate.KeyPath,
		&expiresAt,
		&autoRenew,
		&certificate.RenewBeforeDays,
		&status,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Certificate{}, err
	}

	if err := json.Unmarshal([]byte(domainsJSON), &certificate.Domains); err != nil {
		return domain.Certificate{}, fmt.Errorf("parse domains: %w", err)
	}
	if err := json.Unmarshal([]byte(snisJSON), &certificate.SNIs); err != nil {
		return domain.Certificate{}, fmt.Errorf("parse snis: %w", err)
	}

	certificate.AutoRenew = autoRenew == 1
	certificate.Status = domain.CertificateStatus(status)
	certificate.CreatedAt, _ = parseSQLiteTime(createdAt)
	certificate.UpdatedAt, _ = parseSQLiteTime(updatedAt)
	if expiresAt.Valid {
		parsed, err := parseSQLiteTime(expiresAt.String)
		if err != nil {
			return domain.Certificate{}, fmt.Errorf("parse expires_at: %w", err)
		}
		certificate.ExpiresAt = &parsed
	}

	return certificate, nil
}

func scanCertificateKongTarget(scanner interface {
	Scan(dest ...any) error
}) (domain.CertificateKongTarget, error) {
	var mapping domain.CertificateKongTarget
	var syncStatus string
	var lastSyncedAt sql.NullString
	var createdAt string
	var updatedAt string

	err := scanner.Scan(
		&mapping.ID,
		&mapping.CertificateID,
		&mapping.KongTargetID,
		&mapping.KongCertificateID,
		&syncStatus,
		&lastSyncedAt,
		&mapping.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.CertificateKongTarget{}, err
	}

	mapping.SyncStatus = domain.SyncStatus(syncStatus)
	mapping.CreatedAt, _ = parseSQLiteTime(createdAt)
	mapping.UpdatedAt, _ = parseSQLiteTime(updatedAt)
	if lastSyncedAt.Valid {
		parsed, err := parseSQLiteTime(lastSyncedAt.String)
		if err != nil {
			return domain.CertificateKongTarget{}, fmt.Errorf("parse last_synced_at: %w", err)
		}
		mapping.LastSyncedAt = &parsed
	}

	return mapping, nil
}

func parseSQLiteTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time %q", value)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
