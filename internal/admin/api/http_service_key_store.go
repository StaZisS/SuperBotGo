package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const serviceKeyPrefix = "sbsk_"

type ServiceKeyScope struct {
	PluginID    string `json:"plugin_id"`
	TriggerName string `json:"trigger_name"`
}

type ServiceKeyRecord struct {
	ID         int64             `json:"id"`
	PublicID   string            `json:"public_id"`
	Name       string            `json:"name"`
	Active     bool              `json:"active"`
	ExpiresAt  *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt *time.Time        `json:"last_used_at,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Scopes     []ServiceKeyScope `json:"scopes"`
}

type CreatedServiceKey struct {
	ServiceKeyRecord
	Token string `json:"token"`
}

type ServiceKeyPrincipal struct {
	ID       int64
	PublicID string
	Name     string
}

type ServiceKeyStore interface {
	ListServiceKeys(ctx context.Context) ([]ServiceKeyRecord, error)
	CreateServiceKey(ctx context.Context, name string, scopes []ServiceKeyScope, expiresAt *time.Time) (CreatedServiceKey, error)
	DeleteServiceKey(ctx context.Context, id int64) error
	AuthenticateServiceKey(ctx context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error)
}

type PgServiceKeyStore struct {
	pool *pgxpool.Pool
}

func NewPgServiceKeyStore(pool *pgxpool.Pool) *PgServiceKeyStore {
	return &PgServiceKeyStore{pool: pool}
}

func (s *PgServiceKeyStore) ListServiceKeys(ctx context.Context) ([]ServiceKeyRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, public_id, name, active, expires_at, last_used_at, created_at, updated_at
		FROM http_service_keys
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list service keys: %w", err)
	}
	defer rows.Close()

	var records []ServiceKeyRecord
	keyIDs := make([]int64, 0)
	for rows.Next() {
		var rec ServiceKeyRecord
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
			return nil, fmt.Errorf("scan service key: %w", err)
		}
		records = append(records, rec)
		keyIDs = append(keyIDs, rec.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service keys: %w", err)
	}
	if len(records) == 0 {
		return []ServiceKeyRecord{}, nil
	}

	scopeRows, err := s.pool.Query(ctx, `
		SELECT service_key_id, plugin_id, trigger_name
		FROM http_service_key_scopes
		WHERE service_key_id = ANY($1)
		ORDER BY plugin_id, trigger_name
	`, keyIDs)
	if err != nil {
		return nil, fmt.Errorf("list service key scopes: %w", err)
	}
	defer scopeRows.Close()

	scopeMap := make(map[int64][]ServiceKeyScope, len(records))
	for scopeRows.Next() {
		var keyID int64
		var scope ServiceKeyScope
		if err := scopeRows.Scan(&keyID, &scope.PluginID, &scope.TriggerName); err != nil {
			return nil, fmt.Errorf("scan service key scope: %w", err)
		}
		scopeMap[keyID] = append(scopeMap[keyID], scope)
	}
	if err := scopeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service key scopes: %w", err)
	}

	for i := range records {
		records[i].Scopes = scopeMap[records[i].ID]
	}
	return records, nil
}

func (s *PgServiceKeyStore) CreateServiceKey(ctx context.Context, name string, scopes []ServiceKeyScope, expiresAt *time.Time) (CreatedServiceKey, error) {
	publicID, err := randomHex(12)
	if err != nil {
		return CreatedServiceKey{}, fmt.Errorf("generate public id: %w", err)
	}
	secret, err := randomHex(32)
	if err != nil {
		return CreatedServiceKey{}, fmt.Errorf("generate secret: %w", err)
	}
	hash := hashSecret(secret)
	token := serviceKeyPrefix + publicID + "." + secret

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreatedServiceKey{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var rec CreatedServiceKey
	err = tx.QueryRow(ctx, `
		INSERT INTO http_service_keys (public_id, name, secret_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, public_id, name, active, expires_at, last_used_at, created_at, updated_at
	`, publicID, name, hash, expiresAt).Scan(
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
		return CreatedServiceKey{}, fmt.Errorf("insert service key: %w", err)
	}

	rec.Scopes = make([]ServiceKeyScope, 0, len(scopes))
	for _, scope := range scopes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO http_service_key_scopes (service_key_id, plugin_id, trigger_name)
			VALUES ($1, $2, $3)
		`, rec.ID, scope.PluginID, scope.TriggerName); err != nil {
			return CreatedServiceKey{}, fmt.Errorf("insert service key scope: %w", err)
		}
		rec.Scopes = append(rec.Scopes, scope)
	}

	if err := tx.Commit(ctx); err != nil {
		return CreatedServiceKey{}, fmt.Errorf("commit service key tx: %w", err)
	}

	rec.Token = token
	return rec, nil
}

func (s *PgServiceKeyStore) DeleteServiceKey(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM http_service_keys WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete service key %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("service key %d not found", id)
	}
	return nil
}

func (s *PgServiceKeyStore) AuthenticateServiceKey(ctx context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error) {
	publicID, secret, ok := parseServiceKey(rawToken)
	if !ok {
		return ServiceKeyPrincipal{}, false, nil
	}

	var principal ServiceKeyPrincipal
	var secretHash string
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, public_id, name, secret_hash, expires_at
		FROM http_service_keys
		WHERE public_id = $1 AND active = true
	`, publicID).Scan(&principal.ID, &principal.PublicID, &principal.Name, &secretHash, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ServiceKeyPrincipal{}, false, nil
		}
		return ServiceKeyPrincipal{}, false, fmt.Errorf("find service key %q: %w", publicID, err)
	}

	if expiresAt != nil && time.Now().After(*expiresAt) {
		return ServiceKeyPrincipal{}, false, nil
	}
	if subtle.ConstantTimeCompare([]byte(secretHash), []byte(hashSecret(secret))) != 1 {
		return ServiceKeyPrincipal{}, false, nil
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM http_service_key_scopes
			WHERE service_key_id = $1 AND plugin_id = $2 AND trigger_name = $3
		)
	`, principal.ID, pluginID, triggerName).Scan(&exists); err != nil {
		return ServiceKeyPrincipal{}, false, fmt.Errorf("check service key scope: %w", err)
	}
	if !exists {
		return ServiceKeyPrincipal{}, false, nil
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE http_service_keys SET last_used_at = now(), updated_at = now() WHERE id = $1
	`, principal.ID); err != nil {
		return ServiceKeyPrincipal{}, false, fmt.Errorf("update service key last_used_at: %w", err)
	}

	return principal, true, nil
}

func parseServiceKey(rawToken string) (publicID string, secret string, ok bool) {
	if !strings.HasPrefix(rawToken, serviceKeyPrefix) {
		return "", "", false
	}
	payload := strings.TrimPrefix(rawToken, serviceKeyPrefix)
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

var _ ServiceKeyStore = (*PgServiceKeyStore)(nil)
