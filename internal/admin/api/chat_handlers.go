package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type ChatHandler struct {
	store    AdminChatStore
	adapters *channel.AdapterRegistry
}

func NewChatHandler(store AdminChatStore, adapters *channel.AdapterRegistry) *ChatHandler {
	return &ChatHandler{store: store, adapters: adapters}
}

func (h *ChatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/chats", h.handleListChats)
	mux.HandleFunc("POST /api/admin/broadcast", h.handleBroadcast)
}

func (h *ChatHandler) handleListChats(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []model.ChatReference{})
		return
	}

	filter := ChatFilter{
		ChannelType: r.URL.Query().Get("channel_type"),
		ChatKind:    r.URL.Query().Get("chat_kind"),
	}

	chats, err := h.store.ListChats(r.Context(), filter)
	if err != nil {
		slog.Error("admin: failed to list chats", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list chats")
		return
	}
	if chats == nil {
		chats = []model.ChatReference{}
	}
	writeJSON(w, http.StatusOK, chats)
}

type broadcastRequest struct {
	ChatIDs []int64 `json:"chat_ids"`
	Text    string  `json:"text"`
}

type broadcastResult struct {
	ChatID      int64  `json:"chat_id"`
	ChannelType string `json:"channel_type"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

func (h *ChatHandler) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}

	var body broadcastRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	if len(body.ChatIDs) == 0 {
		writeError(w, http.StatusBadRequest, "chat_ids is required")
		return
	}

	allChats, err := h.store.ListChats(r.Context(), ChatFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch chats")
		return
	}

	chatMap := make(map[int64]model.ChatReference, len(allChats))
	for _, c := range allChats {
		chatMap[c.ID] = c
	}

	msg := model.NewTextMessage(body.Text)
	results := make([]broadcastResult, 0, len(body.ChatIDs))

	for _, id := range body.ChatIDs {
		chat, ok := chatMap[id]
		if !ok {
			results = append(results, broadcastResult{
				ChatID: id,
				Status: "error",
				Error:  "chat not found",
			})
			continue
		}

		err := h.adapters.SendToChat(r.Context(), chat.ChannelType, chat.PlatformChatID, msg)
		if err != nil {
			slog.Error("admin: broadcast send failed",
				"chat_id", id,
				"channel_type", chat.ChannelType,
				"platform_chat_id", chat.PlatformChatID,
				"error", err)
			results = append(results, broadcastResult{
				ChatID:      id,
				ChannelType: string(chat.ChannelType),
				Status:      "error",
				Error:       err.Error(),
			})
			continue
		}

		results = append(results, broadcastResult{
			ChatID:      id,
			ChannelType: string(chat.ChannelType),
			Status:      "sent",
		})
	}

	writeJSON(w, http.StatusOK, results)
}
