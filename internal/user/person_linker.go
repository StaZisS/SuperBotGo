package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"SuperBotGo/internal/model"
)

// PersonAutoLinker automatically links persons to global users
// when they share the same external identity (TSU AccountId).
type PersonAutoLinker struct {
	pool *pgxpool.Pool
}

func NewPersonAutoLinker(pool *pgxpool.Pool) *PersonAutoLinker {
	return &PersonAutoLinker{pool: pool}
}

// LinkByExternalID sets persons.global_user_id for a person whose external_id
// matches the given value, if not already linked. Used after TSU OAuth callback.
func (l *PersonAutoLinker) LinkByExternalID(ctx context.Context, globalUserID model.GlobalUserID, externalID string) error {
	_, err := l.pool.Exec(ctx, `
		UPDATE persons SET global_user_id = $1, updated_at = now()
		WHERE external_id = $2 AND global_user_id IS NULL
	`, globalUserID, externalID)
	if err != nil {
		return fmt.Errorf("auto-link person by external_id %s: %w", externalID, err)
	}
	return nil
}
