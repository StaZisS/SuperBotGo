package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
)

type PgUserRepo struct {
	pool *pgxpool.Pool
}

func NewPgUserRepo(pool *pgxpool.Pool) *PgUserRepo {
	return &PgUserRepo{pool: pool}
}

func (r *PgUserRepo) FindByID(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	var u model.GlobalUser
	var tsuID *string
	var profileJSON *string
	var locale *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, tsu_accounts_id, primary_channel, profile_data, locale, role
		FROM global_users WHERE id = $1
	`, id).Scan(&u.ID, &tsuID, &u.PrimaryChannel, &profileJSON, &locale, &u.Role)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user %d: %w", id, err)
	}

	if locale != nil {
		u.Locale = *locale
	}
	if profileJSON != nil && *profileJSON != "" {
		_ = json.Unmarshal([]byte(*profileJSON), &u.ProfileData)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_type, channel_user_id, global_user_id
		FROM channel_accounts WHERE global_user_id = $1
	`, id)
	if err != nil {
		return nil, fmt.Errorf("find accounts for user %d: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var acc model.ChannelAccount
		if err := rows.Scan(&acc.ID, &acc.ChannelType, &acc.ChannelUserID, &acc.GlobalUserID); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		u.Accounts = append(u.Accounts, acc)
	}

	return &u, nil
}

func (r *PgUserRepo) Save(ctx context.Context, user *model.GlobalUser) (*model.GlobalUser, error) {
	var profileStr *string
	if user.ProfileData != nil {
		b, _ := json.Marshal(user.ProfileData)
		s := string(b)
		profileStr = &s
	}

	role := user.Role
	if role == "" {
		role = "USER"
	}

	locale := user.Locale
	if locale == "" {
		locale = "en"
	}

	if user.ID == 0 {
		err := r.pool.QueryRow(ctx, `
			INSERT INTO global_users (tsu_accounts_id, primary_channel, profile_data, locale, role)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, nil, user.PrimaryChannel, profileStr, locale, role).Scan(&user.ID)
		if err != nil {
			return nil, fmt.Errorf("insert user: %w", err)
		}
	} else {
		_, err := r.pool.Exec(ctx, `
			UPDATE global_users
			SET primary_channel = $2, profile_data = $3, locale = $4, role = $5
			WHERE id = $1
		`, user.ID, user.PrimaryChannel, profileStr, locale, role)
		if err != nil {
			return nil, fmt.Errorf("update user %d: %w", user.ID, err)
		}
	}

	return user, nil
}

func (r *PgUserRepo) UpdateLocale(ctx context.Context, userID model.GlobalUserID, locale string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE global_users SET locale = $2 WHERE id = $1`, userID, locale)
	if err != nil {
		return fmt.Errorf("update locale for user %d: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", userID)
	}
	return nil
}

var _ UserRepository = (*PgUserRepo)(nil)

type PgAccountRepo struct {
	pool *pgxpool.Pool
}

func NewPgAccountRepo(pool *pgxpool.Pool) *PgAccountRepo {
	return &PgAccountRepo{pool: pool}
}

func (r *PgAccountRepo) FindByChannelAndPlatformID(ctx context.Context, ct model.ChannelType, platformID model.PlatformUserID) (*model.ChannelAccount, error) {
	var acc model.ChannelAccount
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_type, channel_user_id, global_user_id
		FROM channel_accounts
		WHERE channel_type = $1 AND channel_user_id = $2
	`, ct, platformID).Scan(&acc.ID, &acc.ChannelType, &acc.ChannelUserID, &acc.GlobalUserID)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find account %s/%s: %w", ct, platformID, err)
	}
	return &acc, nil
}

func (r *PgAccountRepo) Save(ctx context.Context, account *model.ChannelAccount) (*model.ChannelAccount, error) {
	if account.ID == 0 {
		err := r.pool.QueryRow(ctx, `
			INSERT INTO channel_accounts (channel_type, channel_user_id, global_user_id)
			VALUES ($1, $2, $3)
			RETURNING id
		`, account.ChannelType, account.ChannelUserID, account.GlobalUserID).Scan(&account.ID)
		if err != nil {
			return nil, fmt.Errorf("insert account: %w", err)
		}
	} else {
		_, err := r.pool.Exec(ctx, `
			UPDATE channel_accounts
			SET channel_type = $2, channel_user_id = $3, global_user_id = $4
			WHERE id = $1
		`, account.ID, account.ChannelType, account.ChannelUserID, account.GlobalUserID)
		if err != nil {
			return nil, fmt.Errorf("update account %d: %w", account.ID, err)
		}
	}
	return account, nil
}

var _ AccountRepository = (*PgAccountRepo)(nil)
