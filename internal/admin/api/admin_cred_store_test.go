package api

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyAdminCredentialError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "global user duplicate",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "admin_credentials_global_user_id_key",
			},
			want: ErrAdminCredentialsAlreadyExist,
		},
		{
			name: "email duplicate",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "admin_credentials_email_key",
			},
			want: ErrAdminEmailAlreadyUsed,
		},
		{
			name: "non unique violation",
			err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "admin_credentials_email_key",
			},
			want: nil,
		},
		{
			name: "wrapped error",
			err: errors.Join(errors.New("outer"), &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "admin_credentials_global_user_id_key",
			}),
			want: ErrAdminCredentialsAlreadyExist,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyAdminCredentialError(tt.err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
