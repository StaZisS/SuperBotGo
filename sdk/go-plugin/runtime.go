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
// appropriate handler (meta / configure / handle_command).
func Run(p Plugin) {
	action := os.Getenv("PLUGIN_ACTION")
	switch action {
	case "meta":
		handleMeta(p)
	case "configure":
		handleConfigure(p)
	case "handle_command":
		handleCommand(p)
	case "step_callback":
		handleStepCallback(p)
	default:
		writeResponse(responseJSON{Error: "unknown action: " + action})
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
			// New node-based command flow.
			reg := make(callbackMap)
			for _, node := range cmd.Nodes {
				cd.Nodes = append(cd.Nodes, node.toNodeDef(cmd.Name, reg))
			}
		} else {
			// Legacy step-based command flow.
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

	data, _ := json.Marshal(meta)
	os.Stdout.Write(data)
}

// ---------------------------------------------------------------------------
// action: configure
// ---------------------------------------------------------------------------

func handleConfigure(p Plugin) {
	config, _ := io.ReadAll(os.Stdin)

	// Parse and store config for handler access.
	if len(config) > 0 {
		_ = json.Unmarshal(config, &configStore)
	}

	if p.OnConfigure != nil {
		if err := p.OnConfigure(config); err != nil {
			writeResponse(responseJSON{Error: err.Error()})
			return
		}
	}
	// No error — silent success (host expects empty or no output).
}

// ---------------------------------------------------------------------------
// action: handle_command
// ---------------------------------------------------------------------------

func handleCommand(p Plugin) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeResponse(responseJSON{Error: "failed to read stdin: " + err.Error()})
		return
	}

	var req commandRequest
	if err := json.Unmarshal(data, &req); err != nil {
		writeResponse(responseJSON{Error: "failed to parse command request: " + err.Error()})
		return
	}

	// Find the matching command handler.
	var handler func(ctx *CommandContext) error
	for i := range p.Commands {
		if p.Commands[i].Name == req.CommandName {
			handler = p.Commands[i].Handler
			break
		}
	}
	if handler == nil {
		writeResponse(responseJSON{Error: "unknown command: " + req.CommandName})
		return
	}

	// Load plugin config from PLUGIN_CONFIG env var.
	var cfg map[string]interface{}
	if raw := os.Getenv("PLUGIN_CONFIG"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}

	ctx := &CommandContext{
		UserID:      req.UserID,
		ChannelType: req.ChannelType,
		ChatID:      req.ChatID,
		CommandName: req.CommandName,
		Params:      req.Params,
		Locale:      req.Locale,
		config:      cfg,
	}

	if err := handler(ctx); err != nil {
		writeResponse(responseJSON{Error: err.Error(), Logs: ctx.logs})
		return
	}

	writeResponse(responseJSON{Status: "ok", Reply: ctx.reply, Logs: ctx.logs, Messages: ctx.messages})
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

	// Rebuild the callback registry from the plugin definition.
	// The traversal is deterministic, so callback names match those from meta.
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

	// Load plugin config from PLUGIN_CONFIG env var.
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

func writeResponse(v responseJSON) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}

func writeCallbackResponse(v stepCallbackResponse) {
	data, _ := json.Marshal(v)
	os.Stdout.Write(data)
}
