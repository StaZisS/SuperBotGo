package trigger

import (
	"context"
	"encoding/json"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	"SuperBotGo/internal/wasm/eventbus"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type eventTestPlugin struct {
	id     string
	events []model.Event
}

func (p *eventTestPlugin) ID() string                           { return p.id }
func (p *eventTestPlugin) Name() string                         { return p.id }
func (p *eventTestPlugin) Version() string                      { return "test" }
func (p *eventTestPlugin) Commands() []*state.CommandDefinition { return nil }
func (p *eventTestPlugin) HandleEvent(_ context.Context, event model.Event) (*model.EventResponse, error) {
	p.events = append(p.events, event)
	return &model.EventResponse{Status: "ok"}, nil
}

func TestRegistryLookupEventSubscribers(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.RegisterTriggers("p1", []wasmrt.TriggerDef{
		{Name: "orders.created", Type: "event", Topic: "orders.created"},
	})

	got := reg.LookupEventSubscribers("orders.created")
	if len(got) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(got))
	}
	if got[0].PluginID != "p1" || got[0].TriggerName != "orders.created" {
		t.Fatalf("unexpected subscription: %+v", got[0])
	}

	reg.UnregisterTriggers("p1")
	if remaining := reg.LookupEventSubscribers("orders.created"); len(remaining) != 0 {
		t.Fatalf("expected subscriptions to be removed, got %d", len(remaining))
	}
}

func TestEventSubscriberHandleRoutesEventTrigger(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.RegisterTriggers("p1", []wasmrt.TriggerDef{
		{Name: "orders.created", Type: "event", Topic: "orders.created"},
	})

	manager := plugin.NewManager()
	receiver := &eventTestPlugin{id: "p1"}
	manager.Register(receiver)

	subscriber := NewEventSubscriber(NewRouter(reg, manager), reg)
	ctx := eventbus.ContextWithPluginID(context.Background(), "publisher")

	if err := subscriber.Handle(ctx, "orders.created", []byte(`{"id":42}`)); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	if len(receiver.events) != 1 {
		t.Fatalf("expected 1 delivered event, got %d", len(receiver.events))
	}

	eventData, err := receiver.events[0].EventTrigger()
	if err != nil {
		t.Fatalf("EventTrigger() error = %v", err)
	}
	if eventData.Topic != "orders.created" {
		t.Fatalf("topic = %q, want %q", eventData.Topic, "orders.created")
	}
	if eventData.Source != "publisher" {
		t.Fatalf("source = %q, want %q", eventData.Source, "publisher")
	}

	var payload map[string]int
	if err := json.Unmarshal(eventData.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["id"] != 42 {
		t.Fatalf("payload[id] = %d, want 42", payload["id"])
	}
}
