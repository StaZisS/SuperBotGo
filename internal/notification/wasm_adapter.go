package notification

import (
	"context"

	"SuperBotGo/internal/model"
)

// WasmNotifier adapts NotifyAPI to the hostapi.Notifier interface,
// converting primitive types to model types.
type WasmNotifier struct {
	api *NotifyAPI
}

func NewWasmNotifier(api *NotifyAPI) *WasmNotifier {
	return &WasmNotifier{api: api}
}

func (w *WasmNotifier) NotifyUser(ctx context.Context, userID int64, text string, priority int) error {
	return w.api.NotifyUser(ctx, model.GlobalUserID(userID), model.NewTextMessage(text), model.NotifyPriority(priority))
}

func (w *WasmNotifier) NotifyUsers(ctx context.Context, userIDs []int64, msg model.Message, priority int) error {
	ids := make([]model.GlobalUserID, 0, len(userIDs))
	for _, userID := range userIDs {
		ids = append(ids, model.GlobalUserID(userID))
	}
	return w.api.NotifyUsers(ctx, ids, msg, model.NotifyPriority(priority))
}

func (w *WasmNotifier) NotifyChat(ctx context.Context, channelType string, chatID string, text string, priority int) error {
	return w.api.NotifyChat(ctx, model.ChannelType(channelType), chatID, model.NewTextMessage(text), model.NotifyPriority(priority))
}

func (w *WasmNotifier) NotifyStudents(ctx context.Context, scope string, targetID int64, msg model.Message, priority int) error {
	return w.api.NotifyStudents(ctx, scope, targetID, msg, model.NotifyPriority(priority))
}
