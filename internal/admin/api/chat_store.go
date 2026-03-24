package api

import (
	"context"
	"fmt"

	"SuperBotGo/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminChatStore interface {
	ListChats(ctx context.Context, filter ChatFilter) ([]model.ChatReference, error)
}

type ChatFilter struct {
	ChannelType string
	ChatKind    string
}

type PgAdminChatStore struct {
	pool *pgxpool.Pool
}

func NewPgAdminChatStore(pool *pgxpool.Pool) *PgAdminChatStore {
	return &PgAdminChatStore{pool: pool}
}

func (s *PgAdminChatStore) ListChats(ctx context.Context, filter ChatFilter) ([]model.ChatReference, error) {
	query := `SELECT id, channel_type, platform_chat_id, chat_kind, COALESCE(title, ''), COALESCE(locale, '') FROM chat_references WHERE 1=1`
	args := []any{}
	idx := 1

	if filter.ChannelType != "" {
		query += fmt.Sprintf(" AND channel_type = $%d", idx)
		args = append(args, filter.ChannelType)
		idx++
	}
	if filter.ChatKind != "" {
		query += fmt.Sprintf(" AND chat_kind = $%d", idx)
		args = append(args, filter.ChatKind)
		idx++
	}

	query += " ORDER BY id"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()

	var chats []model.ChatReference
	for rows.Next() {
		var c model.ChatReference
		if err := rows.Scan(&c.ID, &c.ChannelType, &c.PlatformChatID, &c.ChatKind, &c.Title, &c.Locale); err != nil {
			return nil, fmt.Errorf("scan chat row: %w", err)
		}
		chats = append(chats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat rows: %w", err)
	}
	return chats, nil
}

var _ AdminChatStore = (*PgAdminChatStore)(nil)
