package channel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/state"
)

type UserService interface {
	FindOrCreateUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, username ...string) (*model.GlobalUser, error)
	GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

type StateManager interface {
	Register(pluginID string, def *state.CommandDefinition)
	StartCommand(ctx context.Context, userID model.GlobalUserID, channelType model.ChannelType, chatID string, pluginID string, commandName string, locale string) (*StateResult, error)
	ProcessInput(ctx context.Context, userID model.GlobalUserID, channelType model.ChannelType, chatID string, input model.UserInput, locale string) (*StateResult, error)
	CancelCommand(ctx context.Context, userID model.GlobalUserID, channelType model.ChannelType) error
	IsPreservesDialog(pluginID, commandName string) bool
	GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error)
}

type StateResult struct {
	PluginID    string
	Message     model.Message
	CommandName string
	IsComplete  bool
	Params      model.OptionMap
}

type PluginRegistry interface {
	GetCommandDefinition(commandName string) *state.CommandDefinition
	GetPluginIDByCommand(commandName string) string
	ResolveCommand(input string) (pluginID string, def *state.CommandDefinition, candidates []model.CommandCandidate)
}

type EventRouter interface {
	RouteEvent(ctx context.Context, event model.Event) (*model.EventResponse, error)
}

type Authorizer interface {
	CheckCommand(ctx context.Context, userID model.GlobalUserID, pluginID string, commandName string, requirements *model.RoleRequirements) (bool, error)
}

// FocusTracker tracks per-user last-used plugin for disambiguation sorting.
type FocusTracker interface {
	Record(userID model.GlobalUserID, pluginID string)
	LastPlugin(userID model.GlobalUserID) string
}

type ChannelManager struct {
	userService UserService
	router      EventRouter
	state       StateManager
	plugins     PluginRegistry
	authorizer  Authorizer
	adapters    *AdapterRegistry
	focus       FocusTracker
	logger      *slog.Logger
}

func NewChannelManager(
	userService UserService,
	router EventRouter,
	stateManager StateManager,
	plugins PluginRegistry,
	authorizer Authorizer,
	adapters *AdapterRegistry,
	focus FocusTracker,
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
		authorizer:  authorizer,
		adapters:    adapters,
		focus:       focus,
		logger:      logger,
	}
}

func (m *ChannelManager) RegisterAdapter(adapter ChannelAdapter) {
	m.adapters.Register(adapter)
}

func (m *ChannelManager) OnUpdate(ctx context.Context, u Update) error {
	user, err := m.userService.FindOrCreateUser(ctx, u.ChannelType, u.PlatformUserID, u.Username)
	if err != nil {
		return err
	}

	loc := user.Locale
	if loc == "" {
		loc = locale.Default()
	}

	if err := m.processUpdate(ctx, user, u.ChannelType, u.Input, u.ChatID, loc); err != nil {
		m.handleError(ctx, u.ChannelType, u.ChatID, user.ID, err)
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
		return m.handleCommand(ctx, user.ID, channelType, input, chatID, locale)
	}
	return m.handleInput(ctx, user.ID, channelType, input, chatID, locale)
}

func (m *ChannelManager) handleCommand(
	ctx context.Context,
	userID model.GlobalUserID,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	loc string,
) error {
	rawName := input.CommandName()

	pluginID, def, candidates := m.plugins.ResolveCommand(rawName)

	// Ambiguous alias — send disambiguation message.
	if len(candidates) > 0 {
		msg := m.buildDisambiguationMessage(userID, candidates, loc)
		return m.adapters.SendToChat(ctx, channelType, chatID, msg)
	}

	// Not found.
	if def == nil {
		return errs.NewSilentError(errs.ErrCommandNotFound, rawName)
	}

	commandName := def.Name

	var requirements *model.RoleRequirements
	if def != nil {
		requirements = def.Requirements
	}

	ok, err := m.authorizer.CheckCommand(ctx, userID, pluginID, commandName, requirements)
	if err != nil {
		return err
	}
	if !ok {
		return m.adapters.SendToChat(ctx, channelType, chatID,
			model.NewTextMessage("Access denied. You don't have permission for this command."))
	}

	if !m.state.IsPreservesDialog(pluginID, commandName) {
		_ = m.state.CancelCommand(ctx, userID, channelType)
	}

	result, err := m.state.StartCommand(ctx, userID, channelType, chatID, pluginID, commandName, loc)
	if err != nil {
		return err
	}

	if result.IsComplete {
		m.recordFocus(userID, pluginID)
		req := model.CommandRequest{
			UserID:      userID,
			ChannelType: channelType,
			ChatID:      chatID,
			PluginID:    pluginID,
			CommandName: commandName,
			Params:      result.Params,
			Locale:      loc,
			Files:       extractFiles(input),
		}
		return m.routeCommand(ctx, pluginID, req)
	}

	return m.adapters.SendToChat(ctx, channelType, chatID, result.Message)
}

func (m *ChannelManager) handleInput(
	ctx context.Context,
	userID model.GlobalUserID,
	channelType model.ChannelType,
	input model.UserInput,
	chatID string,
	loc string,
) error {
	result, err := m.state.ProcessInput(ctx, userID, channelType, chatID, input, loc)
	if err != nil {
		return err
	}

	if !result.Message.IsEmpty() {
		if sendErr := m.adapters.SendToChat(ctx, channelType, chatID, result.Message); sendErr != nil {
			return sendErr
		}
	}

	if result.IsComplete {
		pluginID := result.PluginID
		if pluginID == "" {
			// Fallback for dialogs started before the PluginID field existed.
			pluginID = m.plugins.GetPluginIDByCommand(result.CommandName)
		}
		m.recordFocus(userID, pluginID)
		req := model.CommandRequest{
			UserID:      userID,
			PluginID:    pluginID,
			CommandName: result.CommandName,
			Params:      result.Params,
			ChannelType: channelType,
			ChatID:      chatID,
			Locale:      loc,
			Files:       extractFiles(input),
		}
		return m.routeCommand(ctx, pluginID, req)
	}

	return nil
}

// buildDisambiguationMessage builds an options message listing all candidates.
// The candidate whose plugin matches the user's recent focus is placed first
// and marked as probable; the rest are sorted alphabetically by FQ name.
func (m *ChannelManager) buildDisambiguationMessage(userID model.GlobalUserID, candidates []model.CommandCandidate, loc string) model.Message {
	focusPlugin := ""
	if m.focus != nil {
		focusPlugin = m.focus.LastPlugin(userID)
	}

	// Sort: focused candidate first, then alphabetical by FQ name.
	sorted := make([]model.CommandCandidate, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		iFocused := sorted[i].PluginID == focusPlugin && focusPlugin != ""
		jFocused := sorted[j].PluginID == focusPlugin && focusPlugin != ""
		if iFocused != jFocused {
			return iFocused
		}
		return sorted[i].FQName < sorted[j].FQName
	})

	options := make([]model.Option, len(sorted))
	for i, c := range sorted {
		label := c.FQName
		if c.Description != "" {
			label = c.FQName + " — " + c.Description
		}
		if c.PluginID == focusPlugin && focusPlugin != "" && i == 0 {
			label = "⟶ " + label
		}
		options[i] = model.Option{
			Label: label,
			Value: "/" + c.FQName,
		}
	}

	return model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  i18n.Get("disambiguate.prompt", loc),
				Style: model.StylePlain,
			},
			model.OptionsBlock{
				Options: options,
			},
		},
	}
}

func (m *ChannelManager) recordFocus(userID model.GlobalUserID, pluginID string) {
	if m.focus != nil && pluginID != "" {
		m.focus.Record(userID, pluginID)
	}
}

func (m *ChannelManager) routeCommand(ctx context.Context, pluginID string, req model.CommandRequest) error {
	event, err := model.NewMessengerEvent(req, pluginID)
	if err != nil {
		return fmt.Errorf("build messenger event: %w", err)
	}
	resp, err := m.router.RouteEvent(ctx, event)
	if err != nil {
		return err
	}
	if resp != nil && resp.Error != "" {
		return fmt.Errorf("plugin %q command %q: %s", pluginID, req.CommandName, resp.Error)
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
			m.sendErrorReply(ctx, channelType, chatID, userID, msg, err)

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
			m.sendErrorReply(ctx, channelType, chatID, userID, "An error occurred. Please try again.", err)
		}
		return
	}

	m.logger.Error("unexpected error processing update",
		slog.Int64("user_id", int64(userID)),
		slog.Any("error", err))
	m.sendErrorReply(ctx, channelType, chatID, userID, "An error occurred. Please try again.", err)
}

func (m *ChannelManager) sendErrorReply(ctx context.Context, channelType model.ChannelType, chatID string, userID model.GlobalUserID, msg string, originalErr error) {
	if sendErr := m.adapters.SendToChat(ctx, channelType, chatID, model.NewTextMessage(msg)); sendErr != nil {
		m.logger.Error("failed to send error reply to user",
			slog.Int64("user_id", int64(userID)),
			slog.String("chat_id", chatID),
			slog.Any("send_error", sendErr),
			slog.Any("original_error", originalErr))
	}
}

// extractFiles returns file references from a FileInput, or nil for other input types.
func extractFiles(input model.UserInput) []model.FileRef {
	if fi, ok := input.(model.FileInput); ok {
		return fi.Files
	}
	return nil
}
