package db

import (
	"context"
	"fmt"
	"time"

	"claim-watcher/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

type RunStats struct {
	ClaimsChecked         int
	UpcomingClaims        int
	OverdueClaims         int
	NotificationsRecorded int
	EmailSent             bool
	Status                string
	ErrorMessage          *string
}

func New(ctx context.Context, databaseURL string) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	cfg.MaxConns = 3
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) StartRun(ctx context.Context) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO claim_watcher_run (status)
		VALUES ('RUNNING')
		RETURNING id::text
	`).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create watcher run: %w", err)
	}
	return id, nil
}

func (r *Repository) FinishRun(ctx context.Context, runID string, stats RunStats) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE claim_watcher_run
		SET completed_at = NOW(),
		    claims_checked = $2,
		    upcoming_claims = $3,
		    overdue_claims = $4,
		    notifications_recorded = $5,
		    email_sent = $6,
		    status = $7,
		    error_message = $8
		WHERE id = $1::uuid
	`,
		runID,
		stats.ClaimsChecked,
		stats.UpcomingClaims,
		stats.OverdueClaims,
		stats.NotificationsRecorded,
		stats.EmailSent,
		stats.Status,
		stats.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("finish watcher run: %w", err)
	}
	return nil
}

func (r *Repository) CountActiveClaims(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM claim_input
		WHERE status NOT IN ('ABGESCHLOSSEN', 'ABGELEHNT')
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active claims: %w", err)
	}
	return count, nil
}

func (r *Repository) FindDeadlineClaims(ctx context.Context, warningDays int) ([]model.Claim, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		    c.id::text,
		    c.claim_number,
		    c.claim_type::text,
		    c.category::text,
		    c.priority::text,
		    c.status::text,
		    c.deadline,
		    c.claim_amount::text,
		    c.description,
		    c.created_by_name,
		    c.created_by_email,
		    c.responsible_handler_id::text,
		    au.display_name,
		    au.email,
		    CASE
		        WHEN c.deadline < CURRENT_DATE THEN 'OVERDUE'
		        WHEN c.deadline = CURRENT_DATE THEN 'DUE_TODAY'
		        ELSE 'UPCOMING'
		    END AS notification_type,
		    CASE
		        WHEN c.deadline < CURRENT_DATE THEN 'OVERDUE'
		        WHEN c.deadline = CURRENT_DATE THEN 'DUE_TODAY'
		        WHEN c.deadline <= CURRENT_DATE + 7 THEN 'CRITICAL'
		        ELSE 'URGENT'
		    END AS deadline_state,
		    CASE
		        WHEN c.deadline < CURRENT_DATE
		            THEN CURRENT_DATE - c.deadline
		        ELSE NULL
		    END AS days_overdue,
		    CASE
		        WHEN c.deadline >= CURRENT_DATE
		            THEN c.deadline - CURRENT_DATE
		        ELSE NULL
		    END AS days_remaining
		FROM claim_input c
		LEFT JOIN app_user au
		    ON au.id = c.responsible_handler_id
		WHERE c.deadline IS NOT NULL
		  AND c.deadline <= CURRENT_DATE + ($1::integer)
		  AND c.status NOT IN ('ABGESCHLOSSEN', 'ABGELEHNT')
		ORDER BY
		    CASE
		        WHEN c.deadline < CURRENT_DATE THEN 0
		        WHEN c.deadline = CURRENT_DATE THEN 1
		        ELSE 2
		    END,
		    c.deadline ASC,
		    CASE c.priority::text
		        WHEN 'KRITISCH' THEN 1
		        WHEN 'HOCH' THEN 2
		        WHEN 'MITTEL' THEN 3
		        WHEN 'NIEDRIG' THEN 4
		        ELSE 5
		    END,
		    c.claim_number ASC
	`, warningDays)
	if err != nil {
		return nil, fmt.Errorf("query deadline claims: %w", err)
	}
	defer rows.Close()

	claims := make([]model.Claim, 0)
	for rows.Next() {
		var c model.Claim
		if err := rows.Scan(
			&c.ID,
			&c.ClaimNumber,
			&c.ClaimType,
			&c.Category,
			&c.Priority,
			&c.Status,
			&c.Deadline,
			&c.ClaimAmount,
			&c.Description,
			&c.CreatedByName,
			&c.CreatedByEmail,
			&c.ResponsibleHandlerID,
			&c.ResponsibleHandlerName,
			&c.ResponsibleHandlerEmail,
			&c.NotificationType,
			&c.DeadlineState,
			&c.DaysOverdue,
			&c.DaysRemaining,
		); err != nil {
			return nil, fmt.Errorf("scan deadline claim: %w", err)
		}
		claims = append(claims, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deadline claims: %w", err)
	}

	return claims, nil
}

func (r *Repository) RecordNotifications(ctx context.Context, runID, recipient string, warningDays int, claims []model.Claim) (int, error) {
	if len(claims) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin notification transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	recorded := 0
	for _, claim := range claims {
		var threshold any
		if claim.NotificationType == "UPCOMING" {
			threshold = warningDays
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO claim_notification (
			    claim_id,
			    notification_type,
			    threshold_days,
			    recipient,
			    event_group_id
			)
			VALUES ($1::uuid, $2, $3, $4, $5::uuid)
		`, claim.ID, claim.NotificationType, threshold, recipient, runID)
		if err != nil {
			return 0, fmt.Errorf("record notification for %s: %w", claim.ClaimNumber, err)
		}
		recorded++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit notification transaction: %w", err)
	}

	return recorded, nil
}
