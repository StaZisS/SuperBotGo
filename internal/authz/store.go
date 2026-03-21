package authz

import (
	"context"

	"SuperBotGo/internal/model"
)

// RelationEntry is a single expanded relation that a user has to an object
// (including relations inherited through parent traversal).
type RelationEntry struct {
	Relation   string
	ObjectType string
	ObjectID   string
}

// Store provides all read-only data access needed for authorization decisions.
type Store interface {
	// Roles
	GetRoles(ctx context.Context, userID model.GlobalUserID, roleType model.RoleLayer) ([]model.UserRole, error)
	GetAllRoleNames(ctx context.Context, userID model.GlobalUserID) ([]string, error)

	// ReBAC tuples — batch: loads all relations for a user (with parent expansion)
	// in a single query. Used to build an in-memory set for check()/is_member().
	GetAllUserRelations(ctx context.Context, subjectID string) ([]RelationEntry, error)
	GetMemberGroups(ctx context.Context, subjectID string) ([]string, error)

	// Command policy
	GetCommandPolicy(ctx context.Context, pluginID, commandName string) (enabled bool, policyExpr string, found bool, err error)

	// Core user lookups
	GetExternalID(ctx context.Context, userID model.GlobalUserID) (string, error)
	GetUserChannelAndLocale(ctx context.Context, userID model.GlobalUserID) (primaryChannel, locale string, err error)

	// Schema support
	GetDistinctRoleNames(ctx context.Context) []string
}
