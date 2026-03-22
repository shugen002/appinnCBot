package storage

import (
	"context"
	"database/sql"
	"fmt"

	appmodels "github.com/shugen002/appinnCbot/models"
)

type SQLiteSeenRepository struct {
	db *sql.DB
}

func NewSQLiteSeenRepository(db *sql.DB) appmodels.SeenRepository {
	return &SQLiteSeenRepository{db: db}
}

func (r *SQLiteSeenRepository) GetCount(ctx context.Context, chatID, userID int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT count FROM seens WHERE chat_id = ? AND user_id = ?`, chatID, userID).Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("get count: %w", err)
	}
	return count, nil
}

func (r *SQLiteSeenRepository) SetCount(ctx context.Context, chatID, userID, count int64) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO seens (chat_id, user_id, count, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(chat_id, user_id)
DO UPDATE SET count = excluded.count, updated_at = CURRENT_TIMESTAMP;
`, chatID, userID, count)
	if err != nil {
		return fmt.Errorf("set count: %w", err)
	}
	return nil
}

func (r *SQLiteSeenRepository) EnsureAtLeast(ctx context.Context, chatID, userID, minimum int64) (int64, error) {
	if minimum < 0 {
		minimum = 0
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO seens (chat_id, user_id, count, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(chat_id, user_id)
DO UPDATE SET count = CASE WHEN seens.count < excluded.count THEN excluded.count ELSE seens.count END,
              updated_at = CURRENT_TIMESTAMP;
`, chatID, userID, minimum)
	if err != nil {
		return 0, fmt.Errorf("ensure at least: %w", err)
	}
	return r.GetCount(ctx, chatID, userID)
}

func (r *SQLiteSeenRepository) Decrement(ctx context.Context, chatID, userID int64) (int64, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO seens (chat_id, user_id, count, updated_at)
VALUES (?, ?, -1, CURRENT_TIMESTAMP)
ON CONFLICT(chat_id, user_id)
DO UPDATE SET count = seens.count - 1, updated_at = CURRENT_TIMESTAMP;
`, chatID, userID)
	if err != nil {
		return 0, fmt.Errorf("decrement count: %w", err)
	}
	return r.GetCount(ctx, chatID, userID)
}

func (r *SQLiteSeenRepository) Increment(ctx context.Context, chatID, userID int64) (int64, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO seens (chat_id, user_id, count, updated_at)
VALUES (?, ?, 1, CURRENT_TIMESTAMP)
ON CONFLICT(chat_id, user_id)
DO UPDATE SET count = seens.count + 1, updated_at = CURRENT_TIMESTAMP;
`, chatID, userID)
	if err != nil {
		return 0, fmt.Errorf("increment count: %w", err)
	}
	return r.GetCount(ctx, chatID, userID)
}
