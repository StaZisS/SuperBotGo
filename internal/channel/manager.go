package channel

import (
	"context"
	"errors"
	"log/slog"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

// UserService is the interface the ChannelManager uses for user lookups.
type UserService interface {
	FindOrCreateUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID) (*model.GlobalUser, error)
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

// StateManager is the interface the ChannelManager uses for dialog state.
type StateManager interface {
	Register(def *state.CommandDefinition)
	StartCommand(ctx context.Context, userID model.GlobalUserID, channelType model.ChannelType, commandName string, locale string) (*StateResult, error)
	ProcessInput(ctx context.Context, userID model.GlobalUserID, channelType model.ChannelType, input model.UserInput, locale string) (*StateResult, error)
	CancelCommand(ctx context.Context, userID model.GlobalUserID, channelType model.ChannelType) error
}

// StateResult represents the outcome of a dialog step.
type StateResult struct {
	Message     model.Message
	CommandName string
	IsComplete  bool
	Params      model.OptionMap
}

// PluginRegistry is a minimal interface for looking up plugins by command.
type PluginRegistry interface {
	GetCommandDefinition(commandName string) *state.CommandDefinition
}

// UpdateRouterIface routes completed command requests to the correct plugin.
type UpdateRouterIface interface {
	Route(ctx context.Context, req model.CommandRequest) error
}

// RoleChecker checks if a user has the required roles.
type RoleChecker interface {
	CheckAccess(ctx context.Context, userID model.GlobalUserID, user *model.GlobalUser, req *model.RoleRequirements) (bool, error)
}

// ChannelManager orchestrates incoming updates: user lookup, dialog state
// management, role checking, and routing completed commands to plugins.
type ChannelManager struct {
	userService UserService
	router      UpdateRouterIface
	state       StateManager
	plugins     PluginRegistry
	roles       RoleChecker
	adapters    *AdapterRegistry
	logger      *slog.Logger
}

// NewChannelManager creates a ChannelManager with the given dependencies.
func NewChannelManager(
	userService UserService,
	router UpdateRouterIface,
	stateManager StateManager,
	plugins PluginRegistry,
	roles RoleChecker,
	adapters *AdapterRegistry,
	logger *slog.Logger,
) *ChannelManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &ChannelManager{
		userService: userService,
		router:      router,
		state:       stateManager,
		plugins:     plugins,
		roles:       roles,
		adapters:    adapters,
		logger:      logger,
	}
}

// RegisterAdapter registers a platform adapter and wires this manager as its
// update handler (conceptually; Go adapters hold a reference at construction).
func (m *ChannelManager) RegisterAdapter(adapter ChannelAdapter) {
	m.adapters.Register(adapter)
}

// OnUpdate implements the UpdateHandler interface. It is called by platform
// adapters when a user sends a message or presses a button.
func (m *ChannelManager) OnUpdate(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, input model.UserInput, chatID string) error {
	user, err := m.userService.FindOrCreateUser(ctx, channelType, platformUserID)
	if err != nil {
		return err
	}

	locale := user.Locale
	if locale == "" {
		locale = "en"
	}

	if err := m.processUpdate(ctx, user, channelType, input, chatID, locale); err != nil {
		m.handleError(ctx, channelType, chatID, user.ID, err)
	}
	return nil
}

func (m *ChannelManager) processUpdate(
	ctx context.Context,
	user *model.GlobalUser,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	locale string,
) error {
	if input.IsCommand() {
		return m.handleCommand(ctx, user.ID, user, channelType, input, chatID, locale)
	}
	return m.handleInput(ctx, user.ID, channelType, input, chatID, locale)
}

func (m *ChannelManager) handleCommand(
	ctx context.Context,
	userID model.GlobalUserID,
	user *model.GlobalUser,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	locale string,
) error {
	commandName := input.CommandName()

	def := m.plugins.GetCommandDefinition(commandName)
	if def != nil && def.Requirements != nil {
		ok, err := m.roles.CheckAccess(ctx, userID, user, def.Requirements)
		if err != nil {
			return err
		}
		if !ok {
			return m.adapters.SendToChat(ctx, channelType, chatID,
				model.NewTextMessage("Access denied. You don't have permission for this command."))
		}
	}

	_ = m.state.CancelCommand(ctx, userID, channelType)

	result, err := m.state.StartCommand(ctx, userID, channelType, commandName, locale)
	if err != nil {
		return err
	}

	if result.IsComplete {
		return m.router.Route(ctx, model.CommandRequest{
			UserID:      userID,
			ChannelType: channelType,
			ChatID:      chatID,
			CommandName: commandName,
			Params:      result.Params,
			Locale:      locale,
		})
	}

	return m.adapters.SendToChat(ctx, channelType, chatID, result.Message)
}

func (m *ChannelManager) handleInput(
	ctx context.Context,
	userID model.GlobalUserID,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	locale string,
) error {
	result, err := m.state.ProcessInput(ctx, userID, channelType, input, locale)
	if err != nil {
		return err
	}

	if sendErr := m.adapters.SendToChat(ctx, channelType, chatID, result.Message); sendErr != nil {
		return sendErr
	}

	if result.IsComplete {
		return m.router.Route(ctx, model.CommandRequest{
			UserID:      userID,
			CommandName: result.CommandName,
			Params:      result.Params,
			ChannelType: channelType,
			ChatID:      chatID,
			Locale:      locale,
		})
	}

	return nil
}

func (m *ChannelManager) handleError(ctx context.Context, channelType model.ChannelType, chatID string, userID model.GlobalUserID, err error) {
	var appErr *errs.AppError
	if errors.As(err, &appErr) {
		switch appErr.Severity {
		case errs.SeverityUser:
			m.logger.Warn("user error",
				slog.String("code", string(appErr.Code)),
				slog.Int64("user_id", int64(userID)),
				slog.String("message", appErr.Message))
			msg := appErr.Message
			if msg == "" {
				msg = "An error occurred."
			}
			_ = m.adapters.SendToChat(ctx, channelType, chatID, model.NewTextMessage(msg))

		case errs.SeveritySilent:
			m.logger.Debug("silent error",
				slog.String("code", string(appErr.Code)),
				slog.Int64("user_id", int64(userID)),
				slog.String("message", appErr.Message))

		case errs.SeverityInternal:
			m.logger.Error("internal error",
				slog.String("code", string(appErr.Code)),
				slog.Int64("user_id", int64(userID)),
				slog.Any("error", err))
			_ = m.adapters.SendToChat(ctx, channelType, chatID,
				model.NewTextMessage("An error occurred. Please try again."))
		}
		return
	}

	m.logger.Error("unexpected error processing update",
		slog.Int64("user_id", int64(userID)),
		slog.Any("error", err))
	_ = m.adapters.SendToChat(ctx, channelType, chatID,
		model.NewTextMessage("An error occurred. Please try again."))
}
