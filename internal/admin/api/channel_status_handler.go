package api

import (
	"net/http"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

type ChannelStatusConfig struct {
	TelegramConfigured   bool
	DiscordConfigured    bool
	VKConfigured         bool
	MattermostConfigured bool
}

type ChannelStatusHandler struct {
	adapters *channel.AdapterRegistry
	config   ChannelStatusConfig
}

func NewChannelStatusHandler(adapters *channel.AdapterRegistry, cfg ChannelStatusConfig) *ChannelStatusHandler {
	return &ChannelStatusHandler{adapters: adapters, config: cfg}
}

func (h *ChannelStatusHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/channels/status", h.handleStatus)
}

type channelStatusEntry struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"` // "connected" | "disconnected" | "not_configured"
}

func (h *ChannelStatusHandler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	channels := []struct {
		name       string
		ct         model.ChannelType
		configured bool
	}{
		{"Telegram", model.ChannelTelegram, h.config.TelegramConfigured},
		{"Discord", model.ChannelDiscord, h.config.DiscordConfigured},
		{"VK", model.ChannelVK, h.config.VKConfigured},
		{"Mattermost", model.ChannelMattermost, h.config.MattermostConfigured},
	}

	result := make([]channelStatusEntry, 0, len(channels))
	for _, ch := range channels {
		var status string
		switch {
		case !ch.configured:
			status = "not_configured"
		case h.adapters.IsRegistered(ch.ct):
			adapter := h.adapters.Get(ch.ct)
			if sc, ok := adapter.(channel.StatusChecker); ok && sc.Connected() {
				status = "connected"
			} else {
				status = "disconnected"
			}
		default:
			status = "disconnected"
		}
		result = append(result, channelStatusEntry{
			Name:   ch.name,
			Type:   string(ch.ct),
			Status: status,
		})
	}

	writeJSON(w, http.StatusOK, result)
}
