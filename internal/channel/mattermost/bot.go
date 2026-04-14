package mattermost

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	mm "github.com/mattermost/mattermost/server/public/model"
)

type BotConfig struct {
	URL            string
	Token          string
	ActionsURL     string
	ActionsPath    string
	ActionsSecret  string
	CommandURL     string
	CommandPath    string
	CommandTrigger string
	CommandToken   string
}

type Bot struct {
	client         *mm.Client4
	botUserID      string
	actionsURL     string
	actionsPath    string
	actionsSecret  string
	commandURL     string
	commandPath    string
	commandTrigger string
	commandToken   string
	commandNames   []string
	handler        channel.UpdateHandlerFunc
	joinHandler    channel.ChatJoinHandler
	fileStore      filestore.FileStore
	maxFileSize    int64
	logger         *slog.Logger
	connected      atomic.Bool
	lifecycleCtx   context.Context
	knownChats     sync.Map
	commandTokens  sync.Map
}

const (
	actionContextValueKey  = "sb_value"
	actionContextLabelKey  = "sb_label"
	actionContextSecretKey = "sb_secret"
)

type slashCommandRequest struct {
	TeamID      string
	ChannelID   string
	UserID      string
	UserName    string
	Command     string
	Text        string
	Token       string
	TriggerID   string
	RootID      string
	ChannelName string
	TeamDomain  string
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, fs filestore.FileStore, maxFileSize int64, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.URL == "" || cfg.Token == "" {
		return nil, fmt.Errorf("mattermost: url and token are required")
	}
	if (cfg.ActionsURL == "") != (cfg.ActionsSecret == "") {
		return nil, fmt.Errorf("mattermost: actions_url and actions_secret must be set together")
	}

	client := mm.NewAPIv4Client(cfg.URL)
	client.SetToken(cfg.Token)

	me, _, err := client.GetMe(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("mattermost: get bot user: %w", err)
	}

	return &Bot{
		client:         client,
		botUserID:      me.Id,
		actionsURL:     cfg.ActionsURL,
		actionsPath:    defaultMattermostActionsPath(cfg.ActionsPath),
		actionsSecret:  cfg.ActionsSecret,
		commandURL:     cfg.CommandURL,
		commandPath:    defaultMattermostCommandPath(cfg.CommandPath),
		commandTrigger: defaultMattermostCommandTrigger(cfg.CommandTrigger),
		commandToken:   cfg.CommandToken,
		handler:        handler,
		joinHandler:    joinHandler,
		fileStore:      fs,
		maxFileSize:    maxFileSize,
		logger:         logger,
	}, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.client, b.botUserID, &b.connected, b.fileStore, ActionConfig{
		URL:    b.actionsURL,
		Secret: b.actionsSecret,
	})
}

func (b *Bot) RegisterRoutes(mux *http.ServeMux) error {
	if mux == nil {
		return fmt.Errorf("mattermost: mux is nil")
	}

	if b.actionsEnabled() {
		mux.HandleFunc(b.actionsPath, b.handleAction)
	}
	if b.commandsEnabled() {
		mux.HandleFunc(b.commandPath, b.handleCommand)
	}
	return nil
}

func (b *Bot) RegisterCommands(commands []string) {
	if len(commands) == 0 {
		b.commandNames = nil
		return
	}

	uniq := make(map[string]struct{}, len(commands))
	for _, cmd := range commands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		uniq[cmd] = struct{}{}
	}

	if len(uniq) == 0 {
		b.commandNames = nil
		return
	}

	b.commandNames = b.commandNames[:0]
	for cmd := range uniq {
		b.commandNames = append(b.commandNames, cmd)
	}
	sort.Strings(b.commandNames)
}

func (b *Bot) Start(ctx context.Context) error {
	b.lifecycleCtx = ctx
	backoff := time.Second
	wsURL := websocketURL(b.client.URL)

	b.syncSlashCommands(ctx)

	for {
		if ctx.Err() != nil {
			b.connected.Store(false)
			return nil
		}

		ws, err := mm.NewWebSocketClient4(wsURL, b.client.AuthToken)
		if err != nil {
			b.connected.Store(false)
			b.logger.Error("mattermost: websocket connect failed", slog.Any("error", err))
			if sleepWithContext(ctx, backoff) != nil {
				return nil
			}
			backoff = nextBackoff(backoff)
			continue
		}

		backoff = time.Second
		b.connected.Store(true)
		b.logger.Info("Mattermost bot starting")
		go ws.Listen()

		err = b.runLoop(ctx, ws)
		b.connected.Store(false)
		ws.Close()
		if ctx.Err() != nil {
			return nil
		}

		b.logger.Error("mattermost: websocket stopped", slog.Any("error", err))
		if sleepWithContext(ctx, backoff) != nil {
			return nil
		}
		backoff = nextBackoff(backoff)
	}
}

func (b *Bot) syncSlashCommands(ctx context.Context) {
	if b.commandURL == "" {
		return
	}

	teams, _, err := b.client.GetTeamsForUser(ctx, b.botUserID, "")
	if err != nil {
		b.logger.Warn("mattermost: slash command auto-registration failed to list teams",
			slog.String("trigger", b.commandTrigger),
			slog.Any("error", err))
		return
	}

	if len(teams) == 0 {
		b.logger.Warn("mattermost: slash command auto-registration skipped because bot is not a member of any teams",
			slog.String("trigger", b.commandTrigger))
		return
	}

	for _, team := range teams {
		if team == nil || team.Id == "" {
			continue
		}
		b.syncSlashCommandForTeam(ctx, team)
	}
}

func (b *Bot) syncSlashCommandForTeam(ctx context.Context, team *mm.Team) {
	existing, _, err := b.client.ListCommands(ctx, team.Id, true)
	if err == nil {
		if cmd := findCommandByTrigger(existing, b.commandTrigger); cmd != nil {
			if cmd.Token != "" {
				b.commandTokens.Store(team.Id, cmd.Token)
			}
			if cmd.URL != "" && cmd.URL != b.commandURL {
				b.logger.Warn("mattermost: slash command trigger already exists with a different URL",
					slog.String("team", team.Name),
					slog.String("trigger", b.commandTrigger),
					slog.String("existing_url", cmd.URL),
					slog.String("configured_url", b.commandURL))
				return
			}
			b.logger.Info("mattermost: slash command already configured",
				slog.String("team", team.Name),
				slog.String("trigger", b.commandTrigger))
			return
		}
	} else {
		b.logger.Warn("mattermost: failed to list existing slash commands",
			slog.String("team", team.Name),
			slog.String("trigger", b.commandTrigger),
			slog.Any("error", err))
	}

	created, _, err := b.client.CreateCommand(ctx, &mm.Command{
		TeamId:           team.Id,
		Trigger:          b.commandTrigger,
		Method:           mm.CommandMethodPost,
		URL:              b.commandURL,
		AutoComplete:     true,
		AutoCompleteDesc: "Run HITs bot commands",
		AutoCompleteHint: b.commandAutocompleteHint(),
		DisplayName:      "HITs",
		Description:      "Run HITs bot commands",
	})
	if err != nil {
		var appErr *mm.AppError
		if errors.As(err, &appErr) {
			b.logger.Warn("mattermost: slash command auto-registration failed",
				slog.String("team", team.Name),
				slog.String("trigger", b.commandTrigger),
				slog.Int("status", appErr.StatusCode),
				slog.String("error_id", appErr.Id),
				slog.Any("error", err))
			return
		}
		b.logger.Warn("mattermost: slash command auto-registration failed",
			slog.String("team", team.Name),
			slog.String("trigger", b.commandTrigger),
			slog.Any("error", err))
		return
	}

	if created.Token != "" {
		b.commandTokens.Store(team.Id, created.Token)
	}

	b.logger.Info("mattermost: slash command registered",
		slog.String("team", team.Name),
		slog.String("trigger", b.commandTrigger))
}

func findCommandByTrigger(commands []*mm.Command, trigger string) *mm.Command {
	trigger = strings.TrimSpace(strings.TrimPrefix(trigger, "/"))
	for _, cmd := range commands {
		if cmd == nil {
			continue
		}
		if strings.EqualFold(cmd.Trigger, trigger) {
			return cmd
		}
	}
	return nil
}

func (b *Bot) commandAutocompleteHint() string {
	if len(b.commandNames) == 0 {
		return "[start|plugins|resume]"
	}
	if len(b.commandNames) <= 4 {
		return "[" + strings.Join(b.commandNames, "|") + "]"
	}
	return "<command>"
}

func websocketURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	}

	return strings.TrimRight(parsed.String(), "/")
}

func (b *Bot) runLoop(ctx context.Context, ws *mm.WebSocketClient) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ws.ResponseChannel:
			if !ok {
				if ws.ListenError != nil {
					return ws.ListenError
				}
				return fmt.Errorf("mattermost: websocket response channel closed")
			}
		case <-ws.PingTimeoutChannel:
			return fmt.Errorf("mattermost: websocket ping timeout")
		case event, ok := <-ws.EventChannel:
			if !ok {
				if ws.ListenError != nil {
					return ws.ListenError
				}
				return fmt.Errorf("mattermost: websocket event channel closed")
			}
			if event.EventType() != mm.WebsocketEventPosted {
				continue
			}
			b.handlePostedEvent(ctx, event)
		}
	}
}

func (b *Bot) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req mm.PostActionIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		b.logger.Warn("mattermost: bad action payload", slog.Any("error", err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	b.logger.Info("mattermost: action request received",
		slog.String("user", req.UserId),
		slog.String("channel", req.ChannelId),
		slog.String("post_id", req.PostId))

	if !b.validActionSecret(req.Context) {
		b.logger.Warn("mattermost: action secret mismatch",
			slog.String("user", req.UserId),
			slog.String("channel", req.ChannelId),
			slog.String("post_id", req.PostId))
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	value := contextString(req.Context, actionContextValueKey)
	label := contextString(req.Context, actionContextLabelKey)
	if value == "" {
		b.logger.Warn("mattermost: action value missing",
			slog.String("user", req.UserId),
			slog.String("channel", req.ChannelId),
			slog.String("post_id", req.PostId))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if label == "" {
		label = value
	}

	w.WriteHeader(http.StatusOK)

	go b.processAction(req, value, label)
}

func (b *Bot) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		b.logger.Warn("mattermost: bad slash command payload", slog.Any("error", err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	req := slashCommandRequest{
		TeamID:      r.FormValue("team_id"),
		ChannelID:   r.FormValue("channel_id"),
		UserID:      r.FormValue("user_id"),
		UserName:    r.FormValue("user_name"),
		Command:     r.FormValue("command"),
		Text:        r.FormValue("text"),
		Token:       commandRequestToken(r),
		TriggerID:   r.FormValue("trigger_id"),
		RootID:      r.FormValue("root_id"),
		ChannelName: r.FormValue("channel_name"),
		TeamDomain:  r.FormValue("team_domain"),
	}

	b.logger.Info("mattermost: slash command request received",
		slog.String("user", req.UserID),
		slog.String("team", req.TeamID),
		slog.String("channel", req.ChannelID),
		slog.String("command", req.Command),
		slog.String("text", strings.TrimSpace(req.Text)))

	if !b.validCommandToken(req.TeamID, req.Token) {
		b.logger.Warn("mattermost: slash command token mismatch",
			slog.String("user", req.UserID),
			slog.String("team", req.TeamID),
			slog.String("channel", req.ChannelID))
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	value, ok := b.commandText(req)
	if !ok {
		b.logger.Warn("mattermost: slash command payload invalid",
			slog.String("user", req.UserID),
			slog.String("team", req.TeamID),
			slog.String("channel", req.ChannelID),
			slog.String("command", req.Command),
			slog.String("text", strings.TrimSpace(req.Text)))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(mm.CommandResponse{
		ResponseType: mm.CommandResponseTypeEphemeral,
	}); err != nil {
		b.logger.Warn("mattermost: failed to encode slash command ack", slog.Any("error", err))
	}

	go b.processCommand(req, value)
}

func (b *Bot) processAction(req mm.PostActionIntegrationRequest, value, label string) {
	ctx := b.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}

	b.ensureChatRegistered(ctx, req.ChannelId)

	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelMattermost,
		PlatformUserID:   model.PlatformUserID(req.UserId),
		PlatformUpdateID: fmt.Sprintf("mm:action:%s:%s", req.PostId, req.TriggerId),
		Input:            model.CallbackInput{Data: value, Label: label},
		ChatID:           req.ChannelId,
		Username:         req.UserName,
	}); err != nil {
		b.logger.Error("mattermost: error handling action",
			slog.String("user", req.UserId),
			slog.String("channel", req.ChannelId),
			slog.String("post_id", req.PostId),
			slog.String("value", value),
			slog.String("label", label),
			slog.Any("error", err))
		return
	}

	b.logger.Info("mattermost: action handled",
		slog.String("user", req.UserId),
		slog.String("channel", req.ChannelId),
		slog.String("post_id", req.PostId),
		slog.String("value", value),
		slog.String("label", label))
}

func (b *Bot) processCommand(req slashCommandRequest, value string) {
	ctx := b.lifecycleCtx
	if ctx == nil {
		ctx = context.Background()
	}

	b.ensureChatRegistered(ctx, req.ChannelID)

	updateID := fmt.Sprintf("mm:command:%s:%s", req.UserID, req.TriggerID)
	if req.TriggerID == "" {
		updateID = fmt.Sprintf("mm:command:%s:%d", req.UserID, time.Now().UnixNano())
	}

	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelMattermost,
		PlatformUserID:   model.PlatformUserID(req.UserID),
		PlatformUpdateID: updateID,
		Input:            model.TextInput{Text: value},
		ChatID:           req.ChannelID,
		Username:         req.UserName,
	}); err != nil {
		b.logger.Error("mattermost: error handling slash command",
			slog.String("user", req.UserID),
			slog.String("team", req.TeamID),
			slog.String("channel", req.ChannelID),
			slog.String("command", req.Command),
			slog.String("value", value),
			slog.Any("error", err))
		return
	}

	b.logger.Info("mattermost: slash command handled",
		slog.String("user", req.UserID),
		slog.String("team", req.TeamID),
		slog.String("channel", req.ChannelID),
		slog.String("command", req.Command),
		slog.String("value", value))
}

func (b *Bot) handlePostedEvent(ctx context.Context, event *mm.WebSocketEvent) {
	post, err := decodePost(event.GetData()["post"])
	if err != nil {
		b.logger.Error("mattermost: failed to decode post", slog.Any("error", err))
		return
	}
	if post == nil || post.UserId == "" || post.UserId == b.botUserID || post.Type != "" {
		return
	}

	b.ensureChatRegistered(ctx, post.ChannelId)

	input, ok := b.buildInput(ctx, post)
	if !ok {
		return
	}

	username := ""
	if senderName, ok := event.GetData()["sender_name"].(string); ok {
		username = senderName
	}

	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelMattermost,
		PlatformUserID:   model.PlatformUserID(post.UserId),
		PlatformUpdateID: "mm:" + post.Id,
		Input:            input,
		ChatID:           post.ChannelId,
		Username:         username,
	}); err != nil {
		b.logger.Error("mattermost: error handling post",
			slog.String("user", post.UserId),
			slog.String("channel", post.ChannelId),
			slog.Any("error", err))
	}
}

func (b *Bot) buildInput(ctx context.Context, post *mm.Post) (model.UserInput, bool) {
	if refs := b.collectFiles(ctx, post.FileIds); len(refs) > 0 {
		return model.FileInput{Caption: post.Message, Files: refs}, true
	}

	if strings.TrimSpace(post.Message) == "" {
		return nil, false
	}
	return model.TextInput{Text: post.Message}, true
}

func (b *Bot) collectFiles(ctx context.Context, fileIDs []string) []model.FileRef {
	if b.fileStore == nil || len(fileIDs) == 0 {
		return nil
	}

	refs := make([]model.FileRef, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		info, _, err := b.client.GetFileInfo(ctx, fileID)
		if err != nil {
			b.logger.Error("mattermost: get file info failed", slog.String("file_id", fileID), slog.Any("error", err))
			continue
		}
		if b.maxFileSize > 0 && info.Size > b.maxFileSize {
			b.logger.Warn("mattermost: attachment too large, skipping",
				slog.String("file_id", fileID),
				slog.Int64("size", info.Size),
				slog.Int64("max_size", b.maxFileSize))
			continue
		}

		data, _, err := b.client.GetFile(ctx, fileID)
		if err != nil {
			b.logger.Error("mattermost: download file failed", slog.String("file_id", fileID), slog.Any("error", err))
			continue
		}

		meta := fileMetaFromMattermost(info)
		ref, err := b.fileStore.Store(ctx, meta, bytesReader(data))
		if err != nil {
			b.logger.Error("mattermost: store file failed", slog.String("file_id", fileID), slog.Any("error", err))
			continue
		}
		refs = append(refs, ref)
	}

	return refs
}

func (b *Bot) ensureChatRegistered(ctx context.Context, channelID string) {
	if b.joinHandler == nil {
		return
	}

	if _, loaded := b.knownChats.LoadOrStore(channelID, struct{}{}); loaded {
		return
	}

	ch, _, err := b.client.GetChannel(ctx, channelID)
	if err != nil {
		b.knownChats.Delete(channelID)
		b.logger.Error("mattermost: get channel failed", slog.String("channel", channelID), slog.Any("error", err))
		return
	}

	kind := model.ChatKindGroup
	switch ch.Type {
	case mm.ChannelTypeDirect:
		kind = model.ChatKindPrivate
	case mm.ChannelTypeGroup, mm.ChannelTypeOpen, mm.ChannelTypePrivate:
		kind = model.ChatKindGroup
	}

	title := ch.DisplayName
	if title == "" {
		title = ch.Name
	}
	if title == "" {
		title = channelID
	}

	if err := b.joinHandler.OnChatJoin(ctx, model.ChannelMattermost, channelID, kind, title); err != nil {
		b.knownChats.Delete(channelID)
		b.logger.Error("mattermost: register chat failed", slog.String("channel", channelID), slog.Any("error", err))
	}
}

func decodePost(raw any) (*mm.Post, error) {
	if raw == nil {
		return nil, nil
	}

	var data []byte
	switch value := raw.(type) {
	case string:
		data = []byte(value)
	default:
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}

	var post mm.Post
	if err := json.Unmarshal(data, &post); err != nil {
		return nil, err
	}
	return &post, nil
}

func detectMattermostFileType(name, mimeType string) model.FileType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return model.FileTypePhoto
	case strings.HasPrefix(mimeType, "audio/"):
		return model.FileTypeAudio
	case strings.HasPrefix(mimeType, "video/"):
		return model.FileTypeVideo
	}

	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".ogg"):
		return model.FileTypeVoice
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".gif"), strings.HasSuffix(lower, ".webp"):
		return model.FileTypePhoto
	case strings.HasSuffix(lower, ".mp3"), strings.HasSuffix(lower, ".wav"), strings.HasSuffix(lower, ".m4a"):
		return model.FileTypeAudio
	case strings.HasSuffix(lower, ".mp4"), strings.HasSuffix(lower, ".mov"), strings.HasSuffix(lower, ".avi"), strings.HasSuffix(lower, ".webm"):
		return model.FileTypeVideo
	default:
		return model.FileTypeDocument
	}
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current time.Duration) time.Duration {
	if current >= 30*time.Second {
		return 30 * time.Second
	}
	current *= 2
	if current > 30*time.Second {
		return 30 * time.Second
	}
	return current
}

func (b *Bot) actionsEnabled() bool {
	return b.actionsURL != "" && b.actionsSecret != ""
}

func (b *Bot) commandsEnabled() bool {
	return b.commandURL != "" || b.commandToken != ""
}

func (b *Bot) validActionSecret(ctx map[string]any) bool {
	if b.actionsSecret == "" {
		return false
	}
	secret := contextString(ctx, actionContextSecretKey)
	return subtle.ConstantTimeCompare([]byte(secret), []byte(b.actionsSecret)) == 1
}

func (b *Bot) validCommandToken(teamID, token string) bool {
	expected := b.commandTokenForTeam(teamID)
	if expected == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func (b *Bot) commandTokenForTeam(teamID string) string {
	if teamID != "" {
		if token, ok := b.commandTokens.Load(teamID); ok {
			if value, ok := token.(string); ok && value != "" {
				return value
			}
		}
	}
	return b.commandToken
}

func (b *Bot) commandText(req slashCommandRequest) (string, bool) {
	command := strings.TrimSpace(strings.TrimPrefix(req.Command, "/"))
	if b.commandTrigger != "" && !strings.EqualFold(command, b.commandTrigger) {
		return "", false
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		return "/start", true
	}
	if strings.HasPrefix(text, "/") {
		return text, true
	}
	return "/" + text, true
}

func commandRequestToken(r *http.Request) string {
	if token := strings.TrimSpace(r.FormValue("token")); token != "" {
		return token
	}

	const prefix = "Token "
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(auth, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	}
	return ""
}

func contextString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}

	raw, ok := values[key]
	if !ok {
		return ""
	}

	switch value := raw.(type) {
	case nil:
		return ""
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}

func defaultMattermostActionsPath(path string) string {
	if path == "" {
		return "/mattermost/actions"
	}
	return path
}

func defaultMattermostCommandPath(path string) string {
	if path == "" {
		return "/mattermost/command"
	}
	return path
}

func defaultMattermostCommandTrigger(trigger string) string {
	trigger = strings.TrimSpace(strings.TrimPrefix(trigger, "/"))
	if trigger == "" {
		return "hits"
	}
	return trigger
}
