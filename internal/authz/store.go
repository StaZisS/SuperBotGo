package authz

import (
	"context"

	"SuperBotGo/internal/model"
)

type RelationEntry struct {
	Relation   string
	ObjectType string
	ObjectID   string
}

type Store interface {
	GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error)
	GetAllRoleNames(ctx context.Context, userID model.GlobalUserID) ([]string, error)

	GetAllUserRelations(ctx context.Context, subjectID string) ([]RelationEntry, error)
	GetMemberGroups(ctx context.Context, subjectID string) ([]string, error)

	GetCommandPolicy(ctx context.Context, pluginID, commandName string) (enabled bool, policyExpr string, found bool, err error)

	GetExternalID(ctx context.Context, userID model.GlobalUserID) (string, error)
	GetUserChannelAndLocale(ctx context.Context, userID model.GlobalUserID) (primaryChannel, locale string, err error)

	GetDistinctRoleNames(ctx context.Context) []string
}
