package models

import (
	"context"
	"time"
)

type Seen struct {
	ChatID    int64
	UserID    int64
	Count     int64
	UpdatedAt time.Time
}

type SeenRepository interface {
	GetCount(ctx context.Context, chatID, userID int64) (int64, error)
	SetCount(ctx context.Context, chatID, userID, count int64) error
	EnsureAtLeast(ctx context.Context, chatID, userID, minimum int64) (int64, error)
	Decrement(ctx context.Context, chatID, userID int64) (int64, error)
}
