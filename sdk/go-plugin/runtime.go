package wasmplugin

import (
	"encoding/json"
	"io"
	"os"
)

// Run is the entry point for every WASM plugin.
// Call it from main() with a fully populated [Plugin] value.
//
//	func main() { wasmplugin.Run(myPlugin) }
//
// Run reads PLUGIN_ACTION from the environment and dispatches to the
// appropriate handler (meta / configure / handle_event / step_callback).
func Run(p Plugin) {
	action := os.Getenv("PLUGIN_ACTION")
	switch action {
	case "meta":
		handleMeta(p)
	case "configure":
		handleConfigure(p)
	case "handle_event":
		handleEvent(p)
	case "step_callback":
		handleStepCallback(p)
	default:
		writeEventResponse(eventResponseJSON{Error: "unknown action: " + action})
	}
}

// ---------------------------------------------------------------------------
// action: meta
// ---------------------------------------------------------------------------

func handleMeta(p Plugin) {
	meta := pluginMeta{
		ID:         p.ID,
		Name:       p.Name,
		Version:    p.Version,
		SDKVersion: ProtocolVersion,
	}

	if !p.Config.IsEmpty() {
		data, _ := json.Marshal(p.Config)
		meta.ConfigSchema = json.RawMessage(data)
	}

	for _, cmd := range p.Commands {
		cd := commandDef{
			Name:        cmd.Name,
			Description: cmd.Description,
			MinRole:     cmd.MinRole,
		}

		if len(cmd.Nodes) > 0 {
			reg := make(callbackMap)
			for _, node := range cmd.Nodes {
				cd.Nodes = append(cd.Nodes, node.toNodeDef(cmd.Name, reg))
			}
		} else {
			for _, s := range cmd.Steps {
				sd := stepDef{
					Param:      s.Param,
					Prompt:     s.Prompt,
					Validation: s.Validation,
				}
				for _, o := range s.Options {
					sd.Options = append(sd.Options, optionDef{
						Label: o.Label,
						Value: o.Value,
					})
				}
				cd.Steps = append(cd.Steps, sd)
			}
		}

		meta.Commands = append(meta.Commands, cd)
	}

	for _, perm := range p.Permissions {
		meta.Permissions = append(meta.Permissions, permissionDef{
			Key:         perm.Key,
			Description: perm.Description,
			Required:    perm.Required,
		})
	}

	for _, t := range p.Triggers {
		meta.Triggers = append(meta.Triggers, triggerDef{
			Name:        t.Name,
			Type:        t.Type,
			Description: t.Description,
			Path:        t.Path,
			Methods:     t.Methods,
			Schedule:    t.Schedule,
			Topic:       t.Topic,
		})
	}

	data, _ := json.Marshal(meta)
	os.Stdout.Write(data)
}

// ---------------------------------------------------------------------------
// action: configure
// ---------------------------------------------------------------------------

func handleConfigure(p Plugin) {
	config, _ := io.ReadAll(os.Stdin)

	if len(config) > 0 {
		_ = json.Unmarshal(config, &configStore)
	}

	if p.OnConfigure != nil {
		if err := p.OnConfigure(config); err != nil {
			writeEventResponse(eventResponseJSON{Error: err.Error()})
			return
		}
	}
}

// ---------------------------------------------------------------------------
// action: handle_event (unified handler for all trigger types)
// ---------------------------------------------------------------------------

func handleEvent(p Plugin) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeEventResponse(eventResponseJSON{Error: "failed to read stdin: " + err.Error()})
		return
	}

	var req eventRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeEventResponse(eventResponseJSON{Error: "failed to parse event request: " + err.Error()})
		return
	}

	// Load plugin config from PLUGIN_CONFIG env var.
	var cfg map[string]interface{}
	if raw := os.Getenv("PLUGIN_CONFIG"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}

	ctx := &EventContext{
		PluginID:    req.PluginID,
		TriggerType: req.TriggerType,
		TriggerName: req.TriggerName,
		Timestamp:   req.Timestamp,
		config:      cfg,
	}

	// Find handler and parse trigger-specific data.
	var handler func(ctx *EventContext) error

	switch req.TriggerType {
	case "messenger":
		// Messenger command: find command handler by trigger name (= command name).
		var m messengerTriggerData
		if json.Unmarshal(req.Data, &m) == nil {
			ctx.Messenger = &MessengerData{
				UserID:      m.UserID,
				ChannelType: m.ChannelType,
				ChatID:      m.ChatID,
				CommandName: m.CommandName,
				Params:      m.Params,
				Locale:      m.Locale,
			}
		}
		for i := range p.Commands {
			if p.Commands[i].Name == req.TriggerName {
				handler = p.Commands[i].Handler
				break
			}
		}

	case TriggerHTTP:
		var h httpTriggerData
		if json.Unmarshal(req.Data, &h) == nil {
			ctx.HTTP = &HTTPEventData{
				Method:     h.Method,
				Path:       h.Path,
				Query:      h.Query,
				Headers:    h.Headers,
				Body:       h.Body,
				RemoteAddr: h.RemoteAddr,
			}
		}
		for i := range p.Triggers {
			if p.Triggers[i].Name == req.TriggerName {
				handler = p.Triggers[i].Handler
				break
			}
		}

	case TriggerCron:
		var c cronTriggerData
		if json.Unmarshal(req.Data, &c) == nil {
			ctx.Cron = &CronEventData{
				ScheduleName: c.ScheduleName,
				FireTime:     c.FireTime,
			}
		}
		for i := range p.Triggers {
			if p.Triggers[i].Name == req.TriggerName {
				handler = p.Triggers[i].Handler
				break
			}
		}

	case TriggerEvent:
		var e eventTriggerData
		if json.Unmarshal(req.Data, &e) == nil {
			ctx.Event = &EventBusData{
				Topic:   e.Topic,
				Payload: e.Payload,
				Source:  e.Source,
			}
		}
		for i := range p.Triggers {
			if p.Triggers[i].Name == req.TriggerName {
				handler = p.Triggers[i].Handler
				break
			}
		}
	}

	// Fall back to OnEvent if no specific handler found.
	if handler == nil {
		handler = p.OnEvent
	}
	if handler == nil {
		writeEventResponse(eventResponseJSON{Error: "no handler for trigger: " + req.TriggerName})
		return
	}

	if err := handler(ctx); err != nil {
		writeEventResponse(eventResponseJSON{Error: err.Error(), Logs: ctx.logs})
		return
	}

	resp := eventResponseJSON{
		Status:   "ok",
		Reply:    ctx.reply,
		Logs:     ctx.logs,
		Messages: ctx.messages,
	}
	if ctx.httpResp != nil {
		respData, _ := json.Marshal(ctx.httpResp)
		resp.Data = respData
	}
	writeEventResponse(resp)
}

// ---------------------------------------------------------------------------
// action: step_callback
// ---------------------------------------------------------------------------

func handleStepCallback(p Plugin) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeCallbackResponse(stepCallbackResponse{Error: "failed to read stdin: " + err.Error()})
		return
	}

	var req stepCallbackRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeCallbackResponse(stepCallbackResponse{Error: "failed to parse callback request: " + err.Error()})
		return
	}

	reg := make(callbackMap)
	for _, cmd := range p.Commands {
		for _, node := range cmd.Nodes {
			node.toNodeDef(cmd.Name, reg)
		}
	}

	cb, ok := reg[req.Callback]
	if !ok {
		writeCallbackResponse(stepCallbackResponse{Error: "unknown callback: " + req.Callback})
		return
	}

	var cfg map[string]interface{}
	if raw := os.Getenv("PLUGIN_CONFIG"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}

	ctx := &CallbackContext{
		UserID: req.UserID,
		Locale: req.Locale,
		Params: req.Params,
		Page:   req.Page,
		Input:  req.Input,
		config: cfg,
	}

	switch fn := cb.(type) {
	case func(ctx *CallbackContext) bool:
		result := fn(ctx)
		writeCallbackResponse(stepCallbackResponse{Result: &result})
	case func(ctx *CallbackContext) []Option:
		options := fn(ctx)
		defs := make([]optionDef, len(options))
		for i, o := range options {
			defs[i] = optionDef{Label: o.Label, Value: o.Value}
		}
		writeCallbackResponse(stepCallbackResponse{Options: defs})
	case func(ctx *CallbackContext) OptionsPage:
		page := fn(ctx)
		defs := make([]optionDef, len(page.Options))
		for i, o := range page.Options {
			defs[i] = optionDef{Label: o.Label, Value: o.Value}
		}
		writeCallbackResponse(stepCallbackResponse{Options: defs, HasMore: page.HasMore})
	default:
		writeCallbackResponse(stepCallbackResponse{Error: "unsupported callback type"})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeEventResponse(v eventResponseJSON) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}

func writeCallbackResponse(v stepCallbackResponse) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}
