package userhttp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"SuperBotGo/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userTokenPrefix = "sbuk_"

type UserTokenRecord struct {
	ID         int64      `json:"id"`
	PublicID   string     `json:"public_id"`
	Name       string     `json:"name"`
	Active     bool       `json:"active"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type CreatedUserToken struct {
	UserTokenRecord
	Token string `json:"token"`
}

type TokenStore interface {
	ListUserTokens(ctx context.Context, userID model.GlobalUserID) ([]UserTokenRecord, error)
	CreateUserToken(ctx context.Context, userID model.GlobalUserID, name string, expiresAt *time.Time) (CreatedUserToken, error)
	DeleteUserToken(ctx context.Context, userID model.GlobalUserID, id int64) error
	AuthenticateUserToken(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)
}

type PgTokenStore struct {
	pool *pgxpool.Pool
}

func NewPgTokenStore(pool *pgxpool.Pool) *PgTokenStore {
	return &PgTokenStore{pool: pool}
}

func (s *PgTokenStore) ListUserTokens(ctx context.Context, userID model.GlobalUserID) ([]UserTokenRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, public_id, name, active, expires_at, last_used_at, created_at, updated_at
		FROM http_user_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user tokens: %w", err)
	}
	defer rows.Close()

	var records []UserTokenRecord
	for rows.Next() {
		var rec UserTokenRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.PublicID,
			&rec.Name,
			&rec.Active,
			&rec.ExpiresAt,
			&rec.LastUsedAt,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user token: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user tokens: %w", err)
	}
	if len(records) == 0 {
		return []UserTokenRecord{}, nil
	}
	return records, nil
}

func (s *PgTokenStore) CreateUserToken(ctx context.Context, userID model.GlobalUserID, name string, expiresAt *time.Time) (CreatedUserToken, error) {
	publicID, err := randomHex(12)
	if err != nil {
		return CreatedUserToken{}, fmt.Errorf("generate public id: %w", err)
	}
	secret, err := randomHex(32)
	if err != nil {
		return CreatedUserToken{}, fmt.Errorf("generate secret: %w", err)
	}
	hash := hashSecret(secret)
	token := userTokenPrefix + publicID + "." + secret

	var rec CreatedUserToken
	err = s.pool.QueryRow(ctx, `
		INSERT INTO http_user_tokens (user_id, public_id, name, secret_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, public_id, name, active, expires_at, last_used_at, created_at, updated_at
	`, userID, publicID, name, hash, expiresAt).Scan(
		&rec.ID,
		&rec.PublicID,
		&rec.Name,
		&rec.Active,
		&rec.ExpiresAt,
		&rec.LastUsedAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return CreatedUserToken{}, fmt.Errorf("insert user token: %w", err)
	}

	rec.Token = token
	return rec, nil
}

func (s *PgTokenStore) DeleteUserToken(ctx context.Context, userID model.GlobalUserID, id int64) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM http_user_tokens
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		return fmt.Errorf("delete user token %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("token %d not found", id)
	}
	return nil
}

func (s *PgTokenStore) AuthenticateUserToken(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error) {
	publicID, secret, ok := parseUserToken(rawToken)
	if !ok {
		return 0, false, nil
	}

	var userID model.GlobalUserID
	var secretHash string
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, secret_hash, expires_at
		FROM http_user_tokens
		WHERE public_id = $1 AND active = true
	`, publicID).Scan(&userID, &secretHash, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("find user token %q: %w", publicID, err)
	}

	if expiresAt != nil && time.Now().After(*expiresAt) {
		return 0, false, nil
	}
	if subtle.ConstantTimeCompare([]byte(secretHash), []byte(hashSecret(secret))) != 1 {
		return 0, false, nil
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE http_user_tokens SET last_used_at = now(), updated_at = now() WHERE public_id = $1
	`, publicID); err != nil {
		return 0, false, fmt.Errorf("update user token last_used_at: %w", err)
	}

	return userID, true, nil
}

func parseUserToken(rawToken string) (publicID string, secret string, ok bool) {
	if !strings.HasPrefix(rawToken, userTokenPrefix) {
		return "", "", false
	}
	payload := strings.TrimPrefix(rawToken, userTokenPrefix)
	publicID, secret, ok = strings.Cut(payload, ".")
	if !ok || publicID == "" || secret == "" {
		return "", "", false
	}
	return publicID, secret, true
}

func randomHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

var _ TokenStore = (*PgTokenStore)(nil)
