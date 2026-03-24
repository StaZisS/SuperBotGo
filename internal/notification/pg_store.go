package notification

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
)

// PgPrefsRepo is a PostgreSQL-backed implementation of PrefsRepository.
type PgPrefsRepo struct {
	pool *pgxpool.Pool
}

func NewPgPrefsRepo(pool *pgxpool.Pool) *PgPrefsRepo {
	return &PgPrefsRepo{pool: pool}
}

func (r *PgPrefsRepo) GetPrefs(ctx context.Context, userID model.GlobalUserID) (*model.NotificationPrefs, error) {
	var (
		channelPriorityStr string
		prefs              model.NotificationPrefs
	)

	err := r.pool.QueryRow(ctx, `
		SELECT global_user_id, channel_priority, mute_mentions,
		       work_hours_start, work_hours_end, timezone
		FROM notification_prefs WHERE global_user_id = $1
	`, userID).Scan(
		&prefs.GlobalUserID,
		&channelPriorityStr,
		&prefs.MuteMentions,
		&prefs.WorkHoursStart,
		&prefs.WorkHoursEnd,
		&prefs.Timezone,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notification prefs for user %d: %w", userID, err)
	}

	prefs.ChannelPriority = UnmarshalChannelPriority(channelPriorityStr)
	return &prefs, nil
}

func (r *PgPrefsRepo) SavePrefs(ctx context.Context, prefs *model.NotificationPrefs) error {
	channelPriorityStr := MarshalChannelPriority(prefs.ChannelPriority)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_prefs (global_user_id, channel_priority, mute_mentions,
		                                work_hours_start, work_hours_end, timezone)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (global_user_id) DO UPDATE SET
			channel_priority = EXCLUDED.channel_priority,
			mute_mentions    = EXCLUDED.mute_mentions,
			work_hours_start = EXCLUDED.work_hours_start,
			work_hours_end   = EXCLUDED.work_hours_end,
			timezone         = EXCLUDED.timezone
	`, prefs.GlobalUserID, channelPriorityStr, prefs.MuteMentions,
		prefs.WorkHoursStart, prefs.WorkHoursEnd, prefs.Timezone)

	if err != nil {
		return fmt.Errorf("save notification prefs for user %d: %w", prefs.GlobalUserID, err)
	}
	return nil
}

var _ PrefsRepository = (*PgPrefsRepo)(nil)
