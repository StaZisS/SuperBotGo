package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

// Compile-time check: WasmPlugin must satisfy the plugin.Plugin interface.
var _ plugin.Plugin = (*WasmPlugin)(nil)

// ReplyFunc sends a text reply back to the chat that issued the command.
type ReplyFunc func(ctx context.Context, req model.CommandRequest, text string) error

// SendFunc sends a message to an arbitrary chat on the same channel.
type SendFunc func(ctx context.Context, channelType model.ChannelType, chatID string, text string) error

// WasmPlugin wraps a compiled Wasm module and implements the plugin.Plugin interface.
// Each call creates a new one-shot module instance (fast due to AOT compilation).
type WasmPlugin struct {
	compiled *wasmrt.CompiledModule
	meta     wasmrt.PluginMeta
	config   json.RawMessage
	reply    ReplyFunc
	send     SendFunc
}

// ID returns the plugin's unique identifier.
func (wp *WasmPlugin) ID() string {
	return wp.meta.ID
}

// Name returns the human-readable plugin name.
func (wp *WasmPlugin) Name() string {
	return wp.meta.Name
}

// Version returns the plugin version.
func (wp *WasmPlugin) Version() string {
	return wp.meta.Version
}

// SupportedRoles extracts unique roles from the plugin's command definitions.
func (wp *WasmPlugin) SupportedRoles() []string {
	seen := make(map[string]bool)
	var roles []string
	for _, cmd := range wp.meta.Commands {
		if cmd.MinRole != "" && !seen[cmd.MinRole] {
			seen[cmd.MinRole] = true
			roles = append(roles, cmd.MinRole)
		}
	}
	return roles
}

// Commands converts the plugin's command definitions to state.CommandDefinition.
func (wp *WasmPlugin) Commands() []*state.CommandDefinition {
	defs := make([]*state.CommandDefinition, len(wp.meta.Commands))
	for i, cmd := range wp.meta.Commands {
		def := &state.CommandDefinition{
			Name:        cmd.Name,
			Description: cmd.Description,
		}
		if cmd.MinRole != "" {
			def.Requirements = &model.RoleRequirements{
				GlobalRoles: []string{cmd.MinRole},
			}
		}

		allVars := make(map[string]string)
		for _, step := range cmd.Steps {
			for k, v := range step.Vars {
				allVars[k] = v
			}
		}

		for _, step := range cmd.Steps {
			def.Nodes = append(def.Nodes, stepDefToNode(step, wp.config, allVars))
		}
		defs[i] = def
	}
	return defs
}

// stepDefToNode converts a wasm StepDef to a state.StepNode.
// configJSON is the plugin's config, used to resolve {config.key} placeholders.
// customVars are developer-defined variables from Step.Vars, available as {var.key}.
func stepDefToNode(sd wasmrt.StepDef, configJSON json.RawMessage, customVars map[string]string) state.StepNode {
	node := state.StepNode{
		ParamName: sd.Param,
	}

	configVars := parseConfigVars(configJSON)

	prompt := sd.Prompt
	options := sd.Options
	vars := customVars
	node.MessageBuilder = func(ctx state.StepContext) model.Message {
		resolvedPrompt := interpolate(prompt, ctx.Params, configVars, vars)
		if len(options) > 0 {
			opts := make([]model.Option, len(options))
			for i, o := range options {
				opts[i] = model.Option{
					Label: interpolate(o.Label, ctx.Params, configVars, vars),
					Value: o.Value,
				}
			}
			return model.Message{
				Blocks: []model.ContentBlock{
					model.OptionsBlock{Prompt: resolvedPrompt, Options: opts},
				},
			}
		}
		return model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: resolvedPrompt, Style: model.StylePlain},
			},
		}
	}

	if sd.Validation != "" {
		pattern := sd.Validation
		node.Validate = func(input model.UserInput) bool {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return true
			}
			return re.MatchString(input.TextValue())
		}
	}

	return node
}

// interpolate replaces placeholders in text:
//   - {param_name}  → value from collected step params
//   - {config.key}  → value from plugin config
//   - {var.key}     → custom variable from Step.Vars
func interpolate(text string, params model.OptionMap, configVars, customVars map[string]string) string {
	if !strings.Contains(text, "{") {
		return text
	}
	for key, val := range customVars {
		text = strings.ReplaceAll(text, "{var."+key+"}", val)
	}
	for key, val := range configVars {
		text = strings.ReplaceAll(text, "{config."+key+"}", val)
	}
	for key, val := range params {
		text = strings.ReplaceAll(text, "{"+key+"}", val)
	}
	return text
}

// parseConfigVars extracts string values from a JSON config for use in placeholders.
func parseConfigVars(configJSON json.RawMessage) map[string]string {
	if len(configJSON) == 0 {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(configJSON, &raw); err != nil {
		return nil
	}
	vars := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			vars[k] = val
		case float64:
			vars[k] = fmt.Sprintf("%g", val)
		case bool:
			if val {
				vars[k] = "true"
			} else {
				vars[k] = "false"
			}
		}
	}
	return vars
}

// HandleCommand serializes the request, runs a one-shot handle_command action,
// and returns the result.
func (wp *WasmPlugin) HandleCommand(ctx context.Context, req model.CommandRequest) error {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("wasm plugin %q command %q: marshal request: %w", wp.meta.ID, req.CommandName, err)
	}

	result, err := wp.compiled.CallHandleCommand(ctx, reqJSON, wp.config)
	if err != nil {
		return fmt.Errorf("wasm plugin %q command %q: handle_command: %w", wp.meta.ID, req.CommandName, err)
	}

	if len(result) > 0 {
		var resp struct {
			Error string `json:"error"`
			Reply string `json:"reply"`
			Logs  []struct {
				Level string `json:"level"`
				Msg   string `json:"msg"`
			} `json:"logs"`
			Messages []struct {
				ChatID string `json:"chat_id"`
				Text   string `json:"text"`
			} `json:"messages"`
		}
		if json.Unmarshal(result, &resp) == nil {
			for _, l := range resp.Logs {
				if l.Level == "error" {
					slog.Error("wasm plugin log", "plugin", wp.meta.ID, "message", l.Msg)
				} else {
					slog.Info("wasm plugin log", "plugin", wp.meta.ID, "message", l.Msg)
				}
			}

			if resp.Error != "" {
				return fmt.Errorf("wasm plugin %q command %q: %s", wp.meta.ID, req.CommandName, resp.Error)
			}

			if resp.Reply != "" && wp.reply != nil {
				if err := wp.reply(ctx, req, resp.Reply); err != nil {
					return fmt.Errorf("wasm plugin %q command %q: send reply: %w", wp.meta.ID, req.CommandName, err)
				}
			}

			if wp.send != nil {
				for _, m := range resp.Messages {
					if err := wp.send(ctx, req.ChannelType, m.ChatID, m.Text); err != nil {
						slog.Error("wasm plugin send failed", "plugin", wp.meta.ID, "chat_id", m.ChatID, "error", err)
					}
				}
			}
		}
	}

	slog.Debug("wasm: handle_command completed", "plugin", wp.meta.ID, "command", req.CommandName)
	return nil
}

// SetConfig updates the in-memory plugin configuration.
func (wp *WasmPlugin) SetConfig(config json.RawMessage) {
	wp.config = config
}

// Meta returns the cached plugin metadata.
func (wp *WasmPlugin) Meta() wasmrt.PluginMeta {
	return wp.meta
}

// IsWasm returns true, indicating this is a Wasm plugin.
func (wp *WasmPlugin) IsWasm() bool {
	return true
}

// Close releases the compiled module resources.
func (wp *WasmPlugin) Close(ctx context.Context) error {
	return wp.compiled.Close(ctx)
}
