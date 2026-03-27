package authz

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"SuperBotGo/internal/model"
)

const (
	DefaultSubjectCacheTTL = 30 * time.Second
	DefaultPolicyCacheTTL  = 60 * time.Second
)

type commandPolicyKey struct {
	pluginID    string
	commandName string
}

type commandPolicyValue struct {
	enabled    bool
	policyExpr string
	found      bool
}

type Authorizer struct {
	store        Store
	providers    []AttributeProvider
	logger       *slog.Logger
	subjectCache *TTLCache[model.GlobalUserID, *SubjectContext]
	policyCache  *TTLCache[commandPolicyKey, commandPolicyValue]
}

func NewAuthorizer(store Store, logger *slog.Logger, providers ...AttributeProvider) *Authorizer {
	return NewAuthorizerWithTTL(store, logger, DefaultSubjectCacheTTL, DefaultPolicyCacheTTL, providers...)
}

func NewAuthorizerWithTTL(store Store, logger *slog.Logger, subjectTTL, policyTTL time.Duration, providers ...AttributeProvider) *Authorizer {
	if logger == nil {
		logger = slog.Default()
	}

	var sc *TTLCache[model.GlobalUserID, *SubjectContext]
	if subjectTTL > 0 {
		sc = NewTTLCache[model.GlobalUserID, *SubjectContext](subjectTTL)
	}

	var pc *TTLCache[commandPolicyKey, commandPolicyValue]
	if policyTTL > 0 {
		pc = NewTTLCache[commandPolicyKey, commandPolicyValue](policyTTL)
	}

	return &Authorizer{
		store:        store,
		providers:    providers,
		logger:       logger,
		subjectCache: sc,
		policyCache:  pc,
	}
}

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

	enabled, policyExpr, found, err := a.getCommandPolicy(ctx, pluginID, commandName)
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

func (a *Authorizer) CheckAccess(ctx context.Context, userID model.GlobalUserID, _ *model.GlobalUser, req *model.RoleRequirements) (bool, error) {
	return a.checkRoles(ctx, userID, req)
}

func (a *Authorizer) CanExecute(ctx context.Context, pluginID, commandName string, userID model.GlobalUserID) (bool, error) {
	return a.CheckCommand(ctx, userID, pluginID, commandName, nil)
}

func (a *Authorizer) EvalPolicy(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error) {
	sc, err := a.buildSubjectContext(ctx, userID)
	if err != nil {
		return false, err
	}

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

// InvalidateUser removes the cached SubjectContext for a user.
func (a *Authorizer) InvalidateUser(userID model.GlobalUserID) {
	if a.subjectCache != nil {
		a.subjectCache.Delete(userID)
	}
}

// InvalidateCommandPolicy removes the cached policy for a command.
func (a *Authorizer) InvalidateCommandPolicy(pluginID, commandName string) {
	if a.policyCache != nil {
		a.policyCache.Delete(commandPolicyKey{pluginID, commandName})
	}
}

// ClearCaches clears all authorization caches.
func (a *Authorizer) ClearCaches() {
	if a.subjectCache != nil {
		a.subjectCache.Clear()
	}
	if a.policyCache != nil {
		a.policyCache.Clear()
	}
}

func (a *Authorizer) getCommandPolicy(ctx context.Context, pluginID, commandName string) (bool, string, bool, error) {
	key := commandPolicyKey{pluginID, commandName}

	if a.policyCache != nil {
		if cached, ok := a.policyCache.Get(key); ok {
			return cached.enabled, cached.policyExpr, cached.found, nil
		}
	}

	enabled, policyExpr, found, err := a.store.GetCommandPolicy(ctx, pluginID, commandName)
	if err != nil {
		return false, "", false, err
	}

	if a.policyCache != nil {
		a.policyCache.Set(key, commandPolicyValue{enabled, policyExpr, found})
	}
	return enabled, policyExpr, found, nil
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
	if a.subjectCache != nil {
		if cached, ok := a.subjectCache.Get(userID); ok {
			return cached, nil
		}
	}

	sc := &SubjectContext{
		UserID: userID,
		Attrs:  make(map[string]any),
	}

	var (
		extID   string
		roles   []string
		ch, loc string
	)

	// Phase 1: independent queries in parallel.
	g1, ctx1 := errgroup.WithContext(ctx)

	g1.Go(func() error {
		var err error
		extID, err = a.store.GetExternalID(ctx1, userID)
		if err != nil {
			a.logger.Warn("failed to get external_id", slog.Any("error", err))
		}
		return nil
	})

	g1.Go(func() error {
		var err error
		roles, err = a.store.GetAllRoleNames(ctx1, userID)
		if err != nil {
			a.logger.Warn("failed to get roles", slog.Any("error", err))
		}
		return nil
	})

	g1.Go(func() error {
		var err error
		ch, loc, err = a.store.GetUserChannelAndLocale(ctx1, userID)
		if err != nil {
			a.logger.Warn("failed to get channel/locale", slog.Any("error", err))
		}
		return nil
	})

	_ = g1.Wait()

	sc.ExternalID = extID
	sc.Roles = roles
	sc.PrimaryChannel = ch
	sc.Locale = loc

	// Phase 2: queries that require ExternalID.
	if sc.ExternalID != "" {
		g2, ctx2 := errgroup.WithContext(ctx)

		g2.Go(func() error {
			groups, err := a.store.GetMemberGroups(ctx2, sc.ExternalID)
			if err != nil {
				a.logger.Warn("failed to get member groups", slog.Any("error", err))
			}
			sc.Groups = groups
			return nil
		})

		// Attribute providers run sequentially to avoid races on sc.Attrs.
		g2.Go(func() error {
			for _, p := range a.providers {
				if err := p.LoadAttributes(ctx2, sc); err != nil {
					a.logger.Warn("attribute provider failed", slog.Any("error", err))
				}
			}
			return nil
		})

		_ = g2.Wait()
	}

	if a.subjectCache != nil {
		a.subjectCache.Set(userID, sc)
	}

	return sc, nil
}
