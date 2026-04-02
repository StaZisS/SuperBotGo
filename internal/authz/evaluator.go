package authz

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
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

func buildUserMap(sc *SubjectContext) map[string]any {
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
	return userMap
}

func buildExprEnv(ctx context.Context, sc *SubjectContext, client *authzed.Client) exprEnv {
	roleSet := make(map[string]bool, len(sc.Roles))
	for _, r := range sc.Roles {
		roleSet[r] = true
	}

	subjectID := sc.ExternalID
	if subjectID == "" {
		subjectID = strconv.FormatInt(int64(sc.UserID), 10)
	}
	subject := &v1.SubjectReference{
		Object: &v1.ObjectReference{
			ObjectType: "user",
			ObjectId:   subjectID,
		},
	}

	checkSpice := func(permission, objectType, objectID string) bool {
		if client == nil {
			return false
		}
		resp, err := client.CheckPermission(ctx, &v1.CheckPermissionRequest{
			Resource:   &v1.ObjectReference{ObjectType: objectType, ObjectId: objectID},
			Permission: permission,
			Subject:    subject,
		})
		if err != nil {
			return false
		}
		return resp.Permissionship == v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION
	}

	return exprEnv{
		User: buildUserMap(sc),

		Check: checkSpice,

		IsMember: func(objectType, objectID string) bool {
			return checkSpice("member", objectType, objectID)
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

// EvalWithContext evaluates a policy expression using SpiceDB for graph checks.
func EvalWithContext(ctx context.Context, expression string, sc *SubjectContext, client *authzed.Client) (bool, error) {
	env := buildExprEnv(ctx, sc, client)
	return evaluate(expression, env)
}

var compiledExprs sync.Map // expression string -> *vm.Program

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
