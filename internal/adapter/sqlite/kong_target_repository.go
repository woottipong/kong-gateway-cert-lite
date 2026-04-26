package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"kong-cert-lite/internal/domain"
)

type KongTargetRepository struct {
	db *sql.DB
}

func NewKongTargetRepository(db *sql.DB) *KongTargetRepository {
	return &KongTargetRepository{db: db}
}

func (r *KongTargetRepository) List(ctx context.Context) ([]domain.KongTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, environment, admin_url, auth_type, auth_header_name,
		       auth_header_value, status, last_checked_at, created_at, updated_at
		FROM kong_targets
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list kong targets: %w", err)
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
		return nil, fmt.Errorf("iterate kong targets: %w", err)
	}

	return targets, nil
}

func (r *KongTargetRepository) Get(ctx context.Context, id int64) (domain.KongTarget, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, environment, admin_url, auth_type, auth_header_name,
		       auth_header_value, status, last_checked_at, created_at, updated_at
		FROM kong_targets
		WHERE id = ?
	`, id)

	target, err := scanKongTarget(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.KongTarget{}, domain.ErrNotFound
		}
		return domain.KongTarget{}, fmt.Errorf("get kong target: %w", err)
	}

	return target, nil
}

func (r *KongTargetRepository) Create(ctx context.Context, target domain.KongTarget) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO kong_targets (
			name, environment, admin_url, auth_type,
			auth_header_name, auth_header_value, status
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		target.Name,
		target.Environment,
		target.AdminURL,
		string(target.AuthType),
		target.AuthHeaderName,
		target.AuthHeaderValue,
		string(target.Status),
	)
	if err != nil {
		return 0, fmt.Errorf("create kong target: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read kong target id: %w", err)
	}

	return id, nil
}

func (r *KongTargetRepository) Update(ctx context.Context, target domain.KongTarget) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE kong_targets
		SET name = ?,
		    environment = ?,
		    admin_url = ?,
		    auth_type = ?,
		    auth_header_name = ?,
		    auth_header_value = ?,
		    status = ?,
		    last_checked_at = ?,
		    updated_at = datetime('now')
		WHERE id = ?
	`,
		target.Name,
		target.Environment,
		target.AdminURL,
		string(target.AuthType),
		target.AuthHeaderName,
		target.AuthHeaderValue,
		string(target.Status),
		nullableTime(target.LastCheckedAt),
		target.ID,
	)
	if err != nil {
		return fmt.Errorf("update kong target: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *KongTargetRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM kong_targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete kong target: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func scanKongTarget(scanner interface {
	Scan(dest ...any) error
}) (domain.KongTarget, error) {
	var target domain.KongTarget
	var status string
	var authType string
	var lastCheckedAt sql.NullString
	var createdAt string
	var updatedAt string

	err := scanner.Scan(
		&target.ID,
		&target.Name,
		&target.Environment,
		&target.AdminURL,
		&authType,
		&target.AuthHeaderName,
		&target.AuthHeaderValue,
		&status,
		&lastCheckedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.KongTarget{}, err
	}

	target.AuthType = domain.KongTargetAuthType(authType)
	target.Status = domain.KongTargetStatus(status)
	target.CreatedAt, _ = parseSQLiteTime(createdAt)
	target.UpdatedAt, _ = parseSQLiteTime(updatedAt)
	if lastCheckedAt.Valid {
		parsed, err := time.Parse("2006-01-02 15:04:05", lastCheckedAt.String)
		if err != nil {
			return domain.KongTarget{}, fmt.Errorf("parse last_checked_at: %w", err)
		}
		target.LastCheckedAt = &parsed
	}

	return target, nil
}
