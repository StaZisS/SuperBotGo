package api

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type AdminCredential struct {
	ID           int64     `json:"id"`
	GlobalUserID int64     `json:"global_user_id"`
	Email        string    `json:"email"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PgAdminCredStore struct {
	pool *pgxpool.Pool
}

func NewPgAdminCredStore(pool *pgxpool.Pool) *PgAdminCredStore {
	return &PgAdminCredStore{pool: pool}
}

// Authenticate validates email+password and returns the global user ID on success.
func (s *PgAdminCredStore) Authenticate(ctx context.Context, email, password string) (int64, error) {
	var userID int64
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT global_user_id, password_hash FROM admin_credentials WHERE email = $1`, email,
	).Scan(&userID, &hash)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return 0, fmt.Errorf("query credentials: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return 0, fmt.Errorf("invalid credentials")
	}
	return userID, nil
}

// Create creates admin credentials for a global user.
func (s *PgAdminCredStore) Create(ctx context.Context, globalUserID int64, email, password string) (*AdminCredential, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var cred AdminCredential
	err = s.pool.QueryRow(ctx,
		`INSERT INTO admin_credentials (global_user_id, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, global_user_id, email, created_at, updated_at`,
		globalUserID, email, string(hash),
	).Scan(&cred.ID, &cred.GlobalUserID, &cred.Email, &cred.CreatedAt, &cred.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create admin credentials: %w", err)
	}
	return &cred, nil
}

// VerifyPassword checks that the given password matches the stored hash for a user.
func (s *PgAdminCredStore) VerifyPassword(ctx context.Context, globalUserID int64, password string) error {
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT password_hash FROM admin_credentials WHERE global_user_id = $1`, globalUserID,
	).Scan(&hash)
	if err != nil {
		return fmt.Errorf("credentials not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("invalid password")
	}
	return nil
}

// UpdatePassword changes the password for existing admin credentials.
func (s *PgAdminCredStore) UpdatePassword(ctx context.Context, globalUserID int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	result, err := s.pool.Exec(ctx,
		`UPDATE admin_credentials SET password_hash = $2, updated_at = now() WHERE global_user_id = $1`,
		globalUserID, string(hash))
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("admin credentials not found")
	}
	return nil
}

// UpdateEmail changes the email for existing admin credentials.
func (s *PgAdminCredStore) UpdateEmail(ctx context.Context, globalUserID int64, newEmail string) error {
	result, err := s.pool.Exec(ctx,
		`UPDATE admin_credentials SET email = $2, updated_at = now() WHERE global_user_id = $1`,
		globalUserID, newEmail)
	if err != nil {
		return fmt.Errorf("update email: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("admin credentials not found")
	}
	return nil
}

// Delete removes admin credentials for a global user.
func (s *PgAdminCredStore) Delete(ctx context.Context, globalUserID int64) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM admin_credentials WHERE global_user_id = $1`, globalUserID)
	if err != nil {
		return fmt.Errorf("delete admin credentials: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("admin credentials not found")
	}
	return nil
}

// List returns all admin credentials (without password hashes).
func (s *PgAdminCredStore) List(ctx context.Context) ([]AdminCredential, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ac.id, ac.global_user_id, ac.email, ac.created_at, ac.updated_at
		 FROM admin_credentials ac ORDER BY ac.created_at`)
	if err != nil {
		return nil, fmt.Errorf("list admin credentials: %w", err)
	}
	defer rows.Close()

	var creds []AdminCredential
	for rows.Next() {
		var c AdminCredential
		if err := rows.Scan(&c.ID, &c.GlobalUserID, &c.Email, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

// GetByUserID returns credentials for a specific user (without password hash).
func (s *PgAdminCredStore) GetByUserID(ctx context.Context, globalUserID int64) (*AdminCredential, error) {
	var c AdminCredential
	err := s.pool.QueryRow(ctx,
		`SELECT id, global_user_id, email, created_at, updated_at
		 FROM admin_credentials WHERE global_user_id = $1`, globalUserID,
	).Scan(&c.ID, &c.GlobalUserID, &c.Email, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get admin credential: %w", err)
	}
	return &c, nil
}

// HasAny returns true if at least one admin credential exists.
func (s *PgAdminCredStore) HasAny(ctx context.Context) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_credentials)`).Scan(&exists)
	return exists, err
}
