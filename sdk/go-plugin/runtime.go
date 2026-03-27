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
	case "migrate":
		handleMigrate(p)
	default:
		writeEventResponse(eventResponseJSON{Error: "unknown action: " + action})
	}
}

func handleMeta(p Plugin) {
	meta := pluginMeta{
		ID:         p.ID,
		Name:       p.Name,
		Version:    p.Version,
		SDKVersion: ProtocolVersion,
	}

	var dbFields []DatabaseField
	for _, req := range p.Requirements {
		if req.Type == "database" {
			name := req.Name
			if name == "" {
				name = "default"
			}
			dbFields = append(dbFields, DatabaseField{Name: name, Description: req.Description})
		}
	}

	configSchema := p.Config.withDatabases(dbFields)
	if !configSchema.IsEmpty() {
		data, _ := json.Marshal(configSchema)
		meta.ConfigSchema = json.RawMessage(data)
	}

	for _, t := range p.Triggers {
		td := triggerDef{
			Name:        t.Name,
			Type:        t.Type,
			Description: t.Description,
			Path:        t.Path,
			Methods:     t.Methods,
			Schedule:    t.Schedule,
			Topic:       t.Topic,
		}

		if t.Type == TriggerMessenger && len(t.Nodes) > 0 {
			reg := make(callbackMap)
			for _, node := range t.Nodes {
				td.Nodes = append(td.Nodes, node.toNodeDef(t.Name, reg))
			}
		}

		meta.Triggers = append(meta.Triggers, td)
	}

	for _, req := range p.Requirements {
		rd := requirementDef{
			Type:        req.Type,
			Description: req.Description,
			Name:        req.Name,
			Target:      req.Target,
		}
		if !req.Config.IsEmpty() {
			data, _ := json.Marshal(req.Config)
			rd.Config = json.RawMessage(data)
		}
		meta.Requirements = append(meta.Requirements, rd)
	}

	for _, m := range p.Migrations {
		meta.Migrations = append(meta.Migrations, migrationDef{
			Version:     m.Version,
			Description: m.Description,
			Up:          m.Up,
			Down:        m.Down,
		})
	}

	data, _ := json.Marshal(meta)
	os.Stdout.Write(data)
}

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

func handleMigrate(p Plugin) {
	data, _ := io.ReadAll(os.Stdin)

	if p.Migrate == nil {
		writeMigrateResponse(migrateResponse{Status: "ok"})
		return
	}

	var req migrateRequest
	if len(data) > 0 {
		if err := json.Unmarshal(data, &req); err != nil {
			writeMigrateResponse(migrateResponse{Status: "error", Message: "failed to parse migrate request: " + err.Error()})
			return
		}
	}

	ctx := &MigrateContext{
		OldVersion: req.OldVersion,
		NewVersion: req.NewVersion,
	}

	if err := p.Migrate(ctx); err != nil {
		writeMigrateResponse(migrateResponse{Status: "error", Message: err.Error()})
		return
	}

	writeMigrateResponse(migrateResponse{Status: "ok"})
}

func writeMigrateResponse(v migrateResponse) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}

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

	var handler func(ctx *EventContext) error

	switch req.TriggerType {
	case TriggerMessenger:
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

	case TriggerCron:
		var c cronTriggerData
		if json.Unmarshal(req.Data, &c) == nil {
			ctx.Cron = &CronEventData{
				ScheduleName: c.ScheduleName,
				FireTime:     c.FireTime,
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
	}

	for i := range p.Triggers {
		if p.Triggers[i].Name == req.TriggerName {
			handler = p.Triggers[i].Handler
			break
		}
	}

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
		Status:     "ok",
		Reply:      ctx.reply,
		ReplyTexts: ctx.replyTexts,
		Logs:       ctx.logs,
	}
	if ctx.httpResp != nil {
		respData, _ := json.Marshal(ctx.httpResp)
		resp.Data = respData
	}
	writeEventResponse(resp)
}

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
	for _, t := range p.Triggers {
		if t.Type == TriggerMessenger {
			for _, node := range t.Nodes {
				node.toNodeDef(t.Name, reg)
			}
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
			defs[i] = optionDef{Label: o.Label, Labels: o.Labels, Value: o.Value}
		}
		writeCallbackResponse(stepCallbackResponse{Options: defs})
	case func(ctx *CallbackContext) OptionsPage:
		page := fn(ctx)
		defs := make([]optionDef, len(page.Options))
		for i, o := range page.Options {
			defs[i] = optionDef{Label: o.Label, Labels: o.Labels, Value: o.Value}
		}
		writeCallbackResponse(stepCallbackResponse{Options: defs, HasMore: page.HasMore})
	default:
		writeCallbackResponse(stepCallbackResponse{Error: "unsupported callback type"})
	}
}

func writeEventResponse(v eventResponseJSON) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}

func writeCallbackResponse(v stepCallbackResponse) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}
