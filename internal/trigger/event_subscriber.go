package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/wasm/eventbus"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

const eventTriggerTimeout = time.Duration(wasmrt.DefaultHostEventTimeoutSeconds) * time.Second

type EventSubscriber struct {
	router   *Router
	registry *Registry
}

func NewEventSubscriber(router *Router, registry *Registry) *EventSubscriber {
	return &EventSubscriber{
		router:   router,
		registry: registry,
	}
}

func (s *EventSubscriber) Handle(ctx context.Context, topic string, payload []byte) error {
	subscriptions := s.registry.LookupEventSubscribers(topic)
	if len(subscriptions) == 0 {
		return nil
	}

	source := eventbus.SourcePluginIDFromContext(ctx)
	now := time.Now()

	var firstErr error
	for _, subscription := range subscriptions {
		data, err := json.Marshal(model.EventTriggerData{
			Topic:   topic,
			Payload: json.RawMessage(payload),
			Source:  source,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("marshal event payload for topic %q: %w", topic, err)
			}
			continue
		}

		eventCtx, cancel := context.WithTimeout(ctx, eventTriggerTimeout)
		eventCtx = context.WithValue(eventCtx, wasmrt.PluginTimeoutOverrideKey{}, int(eventTriggerTimeout.Seconds()))

		resp, err := s.router.RouteEvent(eventCtx, model.Event{
			ID:          generateID(),
			TriggerType: model.TriggerEvent,
			TriggerName: subscription.TriggerName,
			PluginID:    subscription.PluginID,
			Timestamp:   now.UnixMilli(),
			Data:        data,
		})
		cancel()

		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("dispatch event %q to %s.%s: %w", topic, subscription.PluginID, subscription.TriggerName, err)
			}
			continue
		}
		if resp != nil && resp.Error != "" && firstErr == nil {
			firstErr = fmt.Errorf("plugin %s returned error for event %q: %s", subscription.PluginID, topic, resp.Error)
		}
	}

	return firstErr
}
