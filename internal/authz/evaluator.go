package authz

import (
	"fmt"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type exprEnv struct {
	User map[string]any `expr:"user"`

	Check      func(relation, objectType, objectID string) bool `expr:"check"`
	IsMember   func(objectType, objectID string) bool           `expr:"is_member"`
	HasRole    func(roleName string) bool                       `expr:"has_role"`
	HasAnyRole func(roleNames ...string) bool                   `expr:"has_any_role"`
}

type relationKey struct {
	relation   string
	objectType string
	objectID   string
}

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

// compiledExprs кэширует скомпилированные expr-lang программы.
// Ключ — текст выражения. Выражения неизменяемы при одном и том же тексте,
// поэтому TTL не нужен — записи живут до перезапуска процесса.
var compiledExprs sync.Map // map[string]*vm.Program

func evaluate(expression string, env exprEnv) (bool, error) {
	var program *vm.Program

	if cached, ok := compiledExprs.Load(expression); ok {
		program = cached.(*vm.Program)
	} else {
		compiled, err := expr.Compile(expression, expr.Env(env), expr.AsBool())
		if err != nil {
			return false, fmt.Errorf("compile expression: %w", err)
		}
		compiledExprs.Store(expression, compiled)
		program = compiled
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
