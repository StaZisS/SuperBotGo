package authz

import (
	"fmt"

	"github.com/expr-lang/expr"
)

// exprEnv is the struct exposed to expr-lang policy expressions.
// Fields and functions are backward-compatible with existing expressions:
//
//	user.nationality_type, user.funding_type, check(), is_member(), has_role(), has_any_role()
type exprEnv struct {
	User map[string]any `expr:"user"`

	Check      func(relation, objectType, objectID string) bool `expr:"check"`
	IsMember   func(objectType, objectID string) bool           `expr:"is_member"`
	HasRole    func(roleName string) bool                       `expr:"has_role"`
	HasAnyRole func(roleNames ...string) bool                   `expr:"has_any_role"`
}

// relationKey is a composite key for the in-memory relation set.
type relationKey struct {
	relation   string
	objectType string
	objectID   string
}

// buildExprEnv constructs the expr-lang environment from a SubjectContext
// and a pre-loaded set of relations. All check()/is_member() calls are
// in-memory lookups — zero DB queries during expression evaluation.
func buildExprEnv(sc *SubjectContext, relations []RelationEntry) exprEnv {
	userMap := map[string]any{
		"id":              int64(sc.UserID),
		"external_id":     sc.ExternalID,
		"groups":          sc.Groups,
		"roles":           sc.Roles,
		"primary_channel": sc.PrimaryChannel,
		"locale":          sc.Locale,
	}

	for k, v := range sc.Attrs {
		userMap[k] = v
	}

	// Build in-memory set from prefetched relations.
	relSet := make(map[relationKey]bool, len(relations))
	for _, r := range relations {
		relSet[relationKey{r.Relation, r.ObjectType, r.ObjectID}] = true
	}

	roleSet := make(map[string]bool, len(sc.Roles))
	for _, r := range sc.Roles {
		roleSet[r] = true
	}

	return exprEnv{
		User: userMap,

		Check: func(relation, objectType, objectID string) bool {
			return relSet[relationKey{relation, objectType, objectID}]
		},

		IsMember: func(objectType, objectID string) bool {
			return relSet[relationKey{"member", objectType, objectID}]
		},

		HasRole: func(roleName string) bool {
			return roleSet[roleName]
		},

		HasAnyRole: func(roleNames ...string) bool {
			for _, rn := range roleNames {
				if roleSet[rn] {
					return true
				}
			}
			return false
		},
	}
}

func evaluate(expression string, env exprEnv) (bool, error) {
	program, err := expr.Compile(expression, expr.Env(env), expr.AsBool())
	if err != nil {
		return false, fmt.Errorf("compile expression: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("run expression: %w", err)
	}

	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression must return bool, got %T", result)
	}
	return b, nil
}
