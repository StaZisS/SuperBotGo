package authz

import (
	"context"
	"log/slog"

	"SuperBotGo/internal/model"
)

// Authorizer is the unified authorization facade.
// It replaces both RoleChecker and CommandAccessChecker with a single entry point.
type Authorizer struct {
	store     Store
	providers []AttributeProvider
	logger    *slog.Logger
}

func NewAuthorizer(store Store, logger *slog.Logger, providers ...AttributeProvider) *Authorizer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Authorizer{store: store, providers: providers, logger: logger}
}

// CheckCommand is the single entry point for command authorization.
// It checks both static RoleRequirements and dynamic policy expressions.
func (a *Authorizer) CheckCommand(
	ctx context.Context,
	userID model.GlobalUserID,
	pluginID string,
	commandName string,
	requirements *model.RoleRequirements,
) (bool, error) {
	if requirements != nil {
		ok, err := a.checkRoles(ctx, userID, requirements)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	if pluginID == "" {
		return true, nil
	}

	enabled, policyExpr, found, err := a.store.GetCommandPolicy(ctx, pluginID, commandName)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil
	}
	if !enabled {
		return false, nil
	}

	if policyExpr != "" {
		ok, evalErr := a.EvalPolicy(ctx, policyExpr, userID)
		if evalErr != nil {
			a.logger.Warn("policy expression error",
				slog.String("plugin", pluginID),
				slog.String("command", commandName),
				slog.Any("error", evalErr))
			return false, nil
		}
		return ok, nil
	}

	return true, nil
}

// CheckAccess satisfies the channel.RoleChecker interface for backward compatibility.
func (a *Authorizer) CheckAccess(ctx context.Context, userID model.GlobalUserID, _ *model.GlobalUser, req *model.RoleRequirements) (bool, error) {
	return a.checkRoles(ctx, userID, req)
}

// CanExecute satisfies the channel.CommandAccessChecker interface for backward compatibility.
func (a *Authorizer) CanExecute(ctx context.Context, pluginID, commandName string, userID model.GlobalUserID) (bool, error) {
	return a.CheckCommand(ctx, userID, pluginID, commandName, nil)
}

// EvalPolicy evaluates a raw policy expression against a user's context.
func (a *Authorizer) EvalPolicy(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error) {
	sc, err := a.buildSubjectContext(ctx, userID)
	if err != nil {
		return false, err
	}

	// Prefetch all user relations in one query.
	var relations []RelationEntry
	if sc.ExternalID != "" {
		relations, err = a.store.GetAllUserRelations(ctx, sc.ExternalID)
		if err != nil {
			a.logger.Warn("failed to prefetch relations", slog.Any("error", err))
		}
	}

	env := buildExprEnv(sc, relations)
	return evaluate(expression, env)
}

func (a *Authorizer) checkRoles(ctx context.Context, userID model.GlobalUserID, req *model.RoleRequirements) (bool, error) {
	if req == nil {
		return true, nil
	}

	if req.SystemRole == "" && len(req.GlobalRoles) == 0 && req.PluginID == "" {
		return true, nil
	}

	if req.SystemRole != "" {
		systemRoles, err := a.store.GetRoles(ctx, userID, model.RoleLayerSystem)
		if err != nil {
			return false, err
		}
		found := false
		for _, r := range systemRoles {
			if r.RoleName == req.SystemRole {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}

	if len(req.GlobalRoles) > 0 {
		globalRoles, err := a.store.GetRoles(ctx, userID, model.RoleLayerGlobal)
		if err != nil {
			return false, err
		}
		roleSet := make(map[string]bool, len(globalRoles))
		for _, r := range globalRoles {
			roleSet[r.RoleName] = true
		}
		for _, required := range req.GlobalRoles {
			if !roleSet[required] {
				return false, nil
			}
		}
	}

	if req.PluginID != "" && req.PluginRole != "" {
		a.logger.Warn("plugin role check not yet implemented",
			slog.String("plugin_id", req.PluginID),
			slog.String("plugin_role", req.PluginRole))
		return false, nil
	}

	return true, nil
}

func (a *Authorizer) buildSubjectContext(ctx context.Context, userID model.GlobalUserID) (*SubjectContext, error) {
	sc := &SubjectContext{
		UserID: userID,
		Attrs:  make(map[string]any),
	}

	extID, err := a.store.GetExternalID(ctx, userID)
	if err != nil {
		a.logger.Warn("failed to get external_id", slog.Any("error", err))
	}
	sc.ExternalID = extID

	roleNames, err := a.store.GetAllRoleNames(ctx, userID)
	if err != nil {
		a.logger.Warn("failed to get roles", slog.Any("error", err))
	}
	sc.Roles = roleNames

	if sc.ExternalID != "" {
		groups, err := a.store.GetMemberGroups(ctx, sc.ExternalID)
		if err != nil {
			a.logger.Warn("failed to get member groups", slog.Any("error", err))
		}
		sc.Groups = groups
	}

	ch, loc, err := a.store.GetUserChannelAndLocale(ctx, userID)
	if err != nil {
		a.logger.Warn("failed to get channel/locale", slog.Any("error", err))
	}
	sc.PrimaryChannel = ch
	sc.Locale = loc

	for _, p := range a.providers {
		if err := p.LoadAttributes(ctx, sc); err != nil {
			a.logger.Warn("attribute provider failed", slog.Any("error", err))
		}
	}

	return sc, nil
}
