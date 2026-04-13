package mattermost

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	URL           string
	Token         string
	ActionsURL    string
	ActionsPath   string
	ActionsSecret string
}

type Bot struct {
	client        *mm.Client4
	botUserID     string
	actionsURL    string
	actionsPath   string
	actionsSecret string
	handler       channel.UpdateHandlerFunc
	joinHandler   channel.ChatJoinHandler
	fileStore     filestore.FileStore
	maxFileSize   int64
	logger        *slog.Logger
	connected     atomic.Bool
	lifecycleCtx  context.Context
	knownChats    sync.Map
}

const (
	actionContextValueKey  = "sb_value"
	actionContextLabelKey  = "sb_label"
	actionContextSecretKey = "sb_secret"
)

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
		client:        client,
		botUserID:     me.Id,
		actionsURL:    cfg.ActionsURL,
		actionsPath:   defaultMattermostActionsPath(cfg.ActionsPath),
		actionsSecret: cfg.ActionsSecret,
		handler:       handler,
		joinHandler:   joinHandler,
		fileStore:     fs,
		maxFileSize:   maxFileSize,
		logger:        logger,
	}, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.client, b.botUserID, &b.connected, b.fileStore, ActionConfig{
		URL:    b.actionsURL,
		Secret: b.actionsSecret,
	})
}

func (b *Bot) RegisterRoutes(mux *http.ServeMux) error {
	if !b.actionsEnabled() {
		return nil
	}
	if mux == nil {
		return fmt.Errorf("mattermost: mux is nil")
	}

	mux.HandleFunc(b.actionsPath, b.handleAction)
	return nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.lifecycleCtx = ctx
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			b.connected.Store(false)
			return nil
		}

		ws, err := mm.NewWebSocketClient4(b.client.URL, b.client.AuthToken)
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
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !b.validActionSecret(req.Context) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	value := contextString(req.Context, actionContextValueKey)
	label := contextString(req.Context, actionContextLabelKey)
	if value == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if label == "" {
		label = value
	}

	ctx := r.Context()
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
			slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(mm.PostActionIntegrationResponse{}); err != nil {
		b.logger.Error("mattermost: failed to write action response", slog.Any("error", err))
	}
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

func (b *Bot) validActionSecret(ctx map[string]any) bool {
	if b.actionsSecret == "" {
		return false
	}
	secret := contextString(ctx, actionContextSecretKey)
	return subtle.ConstantTimeCompare([]byte(secret), []byte(b.actionsSecret)) == 1
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
