package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"SuperBotGo/internal/wasm/registry"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type updatePreviewResponse struct {
	CanUpdate  bool                   `json:"can_update"`
	HasChanges bool                   `json:"has_changes"`
	Current    previewPluginInfo      `json:"current"`
	Next       previewPluginInfo      `json:"next"`
	Summary    []updatePreviewSummary `json:"summary"`
	Warnings   []updatePreviewWarning `json:"warnings"`
	Sections   []updatePreviewSection `json:"sections"`
}

type previewPluginInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type updatePreviewSummary struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Current string `json:"current"`
	Next    string `json:"next"`
	Changed bool   `json:"changed"`
}

type updatePreviewWarning struct {
	Code    string `json:"code"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type updatePreviewSection struct {
	Key          string                  `json:"key"`
	Title        string                  `json:"title"`
	Added        int                     `json:"added"`
	Removed      int                     `json:"removed"`
	Changed      int                     `json:"changed"`
	Same         int                     `json:"same"`
	EmptyMessage string                  `json:"empty_message,omitempty"`
	Items        []updatePreviewDiffItem `json:"items"`
}

type updatePreviewDiffItem struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Change string `json:"change"`
}

func (h *AdminHandler) buildUpdatePreview(ctx context.Context, pluginID string, wasmBytes []byte) (updatePreviewResponse, error) {
	currentMeta, err := h.currentPluginMeta(ctx, pluginID)
	if err != nil {
		return updatePreviewResponse{}, err
	}

	nextMeta, err := h.probeUploadedPlugin(ctx, wasmBytes)
	if err != nil {
		return updatePreviewResponse{}, err
	}

	return buildUpdatePreviewResponse(currentMeta, nextMeta), nil
}

func (h *AdminHandler) currentPluginMeta(ctx context.Context, pluginID string) (wasmrt.PluginMeta, error) {
	record, err := h.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return wasmrt.PluginMeta{}, err
	}

	if metaRecord, err := h.store.GetPluginMetadata(ctx, pluginID); err == nil {
		meta, parseErr := pluginMetaFromRecord(metaRecord)
		if parseErr == nil {
			return meta, nil
		}
	}

	if h.loader != nil {
		if wp, ok := h.loader.GetPlugin(pluginID); ok {
			return wp.Meta(), nil
		}
	}

	if h.blobs != nil && record.WasmKey != "" && h.loader != nil {
		wasmBytes, readErr := h.readBlob(ctx, record.WasmKey)
		if readErr == nil {
			meta, probeErr := h.loader.ProbeMetadataFromBytes(ctx, wasmBytes)
			if probeErr == nil {
				return meta, nil
			}
		}
	}

	return wasmrt.PluginMeta{}, fmt.Errorf("plugin %q metadata not found", pluginID)
}

func (h *AdminHandler) readBlob(ctx context.Context, key string) ([]byte, error) {
	rc, err := h.blobs.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get blob %q: %w", key, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read blob %q: %w", key, err)
	}
	return data, nil
}

func buildUpdatePreviewResponse(currentMeta, nextMeta wasmrt.PluginMeta) updatePreviewResponse {
	triggerSection := buildCollectionSection(
		"triggers",
		"Триггеры",
		"Публичный набор триггеров не изменился.",
		currentMeta.Triggers,
		nextMeta.Triggers,
		func(trigger wasmrt.TriggerDef) string {
			return trigger.Type + ":" + trigger.Name
		},
		func(trigger wasmrt.TriggerDef) string {
			if trigger.Type == "messenger" {
				return "/" + trigger.Name
			}
			return fmt.Sprintf("%s (%s)", trigger.Name, trigger.Type)
		},
		describeTrigger,
	)

	requirementSection := buildCollectionSection(
		"requirements",
		"Требования",
		"Требования к host capabilities не изменились.",
		currentMeta.Requirements,
		nextMeta.Requirements,
		func(req wasmrt.RequirementDef) string {
			return strings.Join([]string{req.Type, req.Name, req.Target}, ":")
		},
		func(req wasmrt.RequirementDef) string {
			scope := req.Name
			if scope == "" {
				scope = req.Target
			}
			if scope == "" {
				return req.Type
			}
			return req.Type + ":" + scope
		},
		describeRequirement,
	)

	rpcSection := buildCollectionSection(
		"rpc_methods",
		"RPC методы",
		"Публичный RPC-контракт не изменился.",
		currentMeta.RPCMethods,
		nextMeta.RPCMethods,
		func(method wasmrt.RPCMethodDef) string { return method.Name },
		func(method wasmrt.RPCMethodDef) string { return method.Name },
		func(method wasmrt.RPCMethodDef) string {
			if method.Description == "" {
				return "Описание не указано"
			}
			return method.Description
		},
	)

	schemaSection, currentFieldCount, nextFieldCount, currentRequiredCount, nextRequiredCount := buildConfigSchemaSection(
		currentMeta.ConfigSchema,
		nextMeta.ConfigSchema,
	)

	currentName := currentMeta.Name
	if currentName == "" {
		currentName = currentMeta.ID
	}
	nextName := nextMeta.Name
	if nextName == "" {
		nextName = nextMeta.ID
	}

	sections := []updatePreviewSection{
		triggerSection,
		requirementSection,
		rpcSection,
		schemaSection,
	}

	warnings := buildUpdatePreviewWarnings(currentMeta, nextMeta)
	hasChanges := currentMeta.ID != nextMeta.ID ||
		currentMeta.Name != nextMeta.Name ||
		currentMeta.Version != nextMeta.Version ||
		triggerSection.Added+triggerSection.Removed+triggerSection.Changed > 0 ||
		requirementSection.Added+requirementSection.Removed+requirementSection.Changed > 0 ||
		rpcSection.Added+rpcSection.Removed+rpcSection.Changed > 0 ||
		schemaSection.Added+schemaSection.Removed+schemaSection.Changed > 0

	return updatePreviewResponse{
		CanUpdate:  currentMeta.ID == nextMeta.ID,
		HasChanges: hasChanges,
		Current: previewPluginInfo{
			ID:      currentMeta.ID,
			Name:    currentName,
			Version: currentMeta.Version,
		},
		Next: previewPluginInfo{
			ID:      nextMeta.ID,
			Name:    nextName,
			Version: nextMeta.Version,
		},
		Summary: []updatePreviewSummary{
			buildSummary("version", "Версия", currentMeta.Version, nextMeta.Version),
			buildSummary("triggers", "Триггеры", fmt.Sprintf("%d", len(currentMeta.Triggers)), fmt.Sprintf("%d", len(nextMeta.Triggers))),
			buildSummary("requirements", "Требования", fmt.Sprintf("%d", len(currentMeta.Requirements)), fmt.Sprintf("%d", len(nextMeta.Requirements))),
			buildSummary("rpc_methods", "RPC методы", fmt.Sprintf("%d", len(currentMeta.RPCMethods)), fmt.Sprintf("%d", len(nextMeta.RPCMethods))),
			buildSummary("config_fields", "Поля конфига", fmt.Sprintf("%d", currentFieldCount), fmt.Sprintf("%d", nextFieldCount)),
			buildSummary("config_required", "Обязательные поля", fmt.Sprintf("%d", currentRequiredCount), fmt.Sprintf("%d", nextRequiredCount)),
		},
		Warnings: warnings,
		Sections: sections,
	}
}

func buildSummary(key, title, current, next string) updatePreviewSummary {
	return updatePreviewSummary{
		Key:     key,
		Title:   title,
		Current: current,
		Next:    next,
		Changed: current != next,
	}
}

func buildUpdatePreviewWarnings(currentMeta, nextMeta wasmrt.PluginMeta) []updatePreviewWarning {
	warnings := make([]updatePreviewWarning, 0)

	if currentMeta.ID != nextMeta.ID {
		warnings = append(warnings, updatePreviewWarning{
			Code:    "plugin_id_mismatch",
			Level:   "error",
			Title:   "Другой plugin id",
			Message: fmt.Sprintf("Открыт плагин %q, а в загруженном файле объявлен %q. Такое обновление будет отклонено.", currentMeta.ID, nextMeta.ID),
		})
	}

	if currentMeta.Version == "" || nextMeta.Version == "" {
		return warnings
	}

	switch cmp := registry.CompareVersions(nextMeta.Version, currentMeta.Version); {
	case cmp == 0:
		warnings = append(warnings, updatePreviewWarning{
			Code:    "same_version",
			Level:   "warn",
			Title:   "Та же версия",
			Message: fmt.Sprintf("Сейчас установлена версия %s и в файле тоже %s. Это будет переустановка без повышения версии.", currentMeta.Version, nextMeta.Version),
		})
	case cmp < 0:
		warnings = append(warnings, updatePreviewWarning{
			Code:    "downgrade",
			Level:   "warn",
			Title:   "Откат на старую версию",
			Message: fmt.Sprintf("Сейчас установлена версия %s, а загружена более старая %s.", currentMeta.Version, nextMeta.Version),
		})
	}

	return warnings
}

func buildConfigSchemaSection(currentSchema, nextSchema json.RawMessage) (updatePreviewSection, int, int, int, int) {
	currentProps, currentRequired := schemaFieldsAndRequired(currentSchema)
	nextProps, nextRequired := schemaFieldsAndRequired(nextSchema)

	section := updatePreviewSection{
		Key:          "config_schema",
		Title:        "Схема конфигурации",
		EmptyMessage: "Структура конфигурации не меняется.",
		Items:        make([]updatePreviewDiffItem, 0),
	}

	fieldNames := make([]string, 0, len(currentProps)+len(nextProps))
	fieldSet := make(map[string]struct{}, len(currentProps)+len(nextProps))
	for name := range currentProps {
		fieldSet[name] = struct{}{}
	}
	for name := range nextProps {
		fieldSet[name] = struct{}{}
	}
	for name := range fieldSet {
		fieldNames = append(fieldNames, name)
	}
	slices.Sort(fieldNames)

	for _, name := range fieldNames {
		currentProp, currentOK := currentProps[name]
		nextProp, nextOK := nextProps[name]

		switch {
		case !currentOK && nextOK:
			section.Added++
			section.Items = append(section.Items, updatePreviewDiffItem{
				Key:    "field:add:" + name,
				Title:  name,
				Detail: describeSchemaProperty(nextProp),
				Change: "added",
			})
		case currentOK && !nextOK:
			section.Removed++
			section.Items = append(section.Items, updatePreviewDiffItem{
				Key:    "field:remove:" + name,
				Title:  name,
				Detail: describeSchemaProperty(currentProp),
				Change: "removed",
			})
		case stableJSON(currentProp) != stableJSON(nextProp):
			section.Changed++
			section.Items = append(section.Items, updatePreviewDiffItem{
				Key:    "field:change:" + name,
				Title:  name,
				Before: describeSchemaProperty(currentProp),
				After:  describeSchemaProperty(nextProp),
				Change: "changed",
			})
		default:
			section.Same++
		}
	}

	for _, name := range sortedStringSetDiff(nextRequired, currentRequired) {
		section.Changed++
		section.Items = append(section.Items, updatePreviewDiffItem{
			Key:    "required:add:" + name,
			Title:  name,
			Before: "optional",
			After:  "required",
			Change: "changed",
		})
	}
	for _, name := range sortedStringSetDiff(currentRequired, nextRequired) {
		section.Changed++
		section.Items = append(section.Items, updatePreviewDiffItem{
			Key:    "required:remove:" + name,
			Title:  name,
			Before: "required",
			After:  "optional",
			Change: "changed",
		})
	}

	return section, len(currentProps), len(nextProps), len(currentRequired), len(nextRequired)
}

func schemaFieldsAndRequired(raw json.RawMessage) (map[string]any, map[string]struct{}) {
	resultProps := make(map[string]any)
	resultRequired := make(map[string]struct{})
	if len(raw) == 0 {
		return resultProps, resultRequired
	}

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return resultProps, resultRequired
	}

	if props, ok := schema["properties"].(map[string]any); ok {
		for name, value := range props {
			resultProps[name] = value
		}
	}

	if required, ok := schema["required"].([]any); ok {
		for _, value := range required {
			name, ok := value.(string)
			if ok {
				resultRequired[name] = struct{}{}
			}
		}
	}

	return resultProps, resultRequired
}

func describeSchemaProperty(v any) string {
	prop, ok := v.(map[string]any)
	if !ok {
		return compactJSON(v)
	}

	var parts []string
	if t, ok := prop["type"].(string); ok && t != "" {
		parts = append(parts, "type: "+t)
	}
	if title, ok := prop["title"].(string); ok && title != "" {
		parts = append(parts, title)
	}
	if desc, ok := prop["description"].(string); ok && desc != "" {
		parts = append(parts, desc)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " • ")
	}
	return compactJSON(v)
}

func describeTrigger(trigger wasmrt.TriggerDef) string {
	var parts []string
	switch trigger.Type {
	case "messenger":
		if trigger.Name != "" {
			parts = append(parts, "/"+trigger.Name)
		}
		if trigger.Description != "" {
			parts = append(parts, trigger.Description)
		}
	case "http":
		if len(trigger.Methods) > 0 {
			parts = append(parts, strings.Join(trigger.Methods, ", "))
		}
		if trigger.Path != "" {
			parts = append(parts, trigger.Path)
		}
		if trigger.Description != "" {
			parts = append(parts, trigger.Description)
		}
	case "cron":
		if trigger.Schedule != "" {
			parts = append(parts, trigger.Schedule)
		}
		if trigger.Description != "" {
			parts = append(parts, trigger.Description)
		}
	case "event":
		if trigger.Topic != "" {
			parts = append(parts, trigger.Topic)
		}
		if trigger.Description != "" {
			parts = append(parts, trigger.Description)
		}
	default:
		if trigger.Description != "" {
			parts = append(parts, trigger.Description)
		}
	}
	return strings.Join(parts, " • ")
}

func describeRequirement(req wasmrt.RequirementDef) string {
	var parts []string
	if req.Target != "" {
		parts = append(parts, "target: "+req.Target)
	}
	if req.Description != "" {
		parts = append(parts, req.Description)
	}
	if len(req.Config) > 0 && string(req.Config) != "null" && string(req.Config) != "{}" {
		parts = append(parts, "есть policy config")
	}
	return strings.Join(parts, " • ")
}

func pluginMetaFromRecord(record PluginMetadataRecord) (wasmrt.PluginMeta, error) {
	if len(record.MetaJSON) > 0 {
		var meta wasmrt.PluginMeta
		if err := json.Unmarshal(record.MetaJSON, &meta); err != nil {
			return wasmrt.PluginMeta{}, fmt.Errorf("decode plugin metadata %q: %w", record.PluginID, err)
		}
		return meta, nil
	}

	meta := wasmrt.PluginMeta{
		ID:           record.PluginID,
		Name:         record.Name,
		Version:      record.Version,
		SDKVersion:   record.SDKVersion,
		ConfigSchema: cloneJSON(record.ConfigSchema),
	}
	if len(record.Requirements) > 0 {
		_ = json.Unmarshal(record.Requirements, &meta.Requirements)
	}
	if len(record.Triggers) > 0 {
		_ = json.Unmarshal(record.Triggers, &meta.Triggers)
	}
	return meta, nil
}

func buildCollectionSection[T any](
	key string,
	title string,
	emptyMessage string,
	currentItems []T,
	nextItems []T,
	keyOf func(T) string,
	labelOf func(T) string,
	detailOf func(T) string,
) updatePreviewSection {
	section := updatePreviewSection{
		Key:          key,
		Title:        title,
		EmptyMessage: emptyMessage,
		Items:        make([]updatePreviewDiffItem, 0),
	}

	currentMap := make(map[string]T, len(currentItems))
	nextMap := make(map[string]T, len(nextItems))
	keys := make([]string, 0, len(currentItems)+len(nextItems))
	seen := make(map[string]struct{}, len(currentItems)+len(nextItems))

	for _, item := range currentItems {
		itemKey := keyOf(item)
		currentMap[itemKey] = item
		if _, ok := seen[itemKey]; !ok {
			keys = append(keys, itemKey)
			seen[itemKey] = struct{}{}
		}
	}
	for _, item := range nextItems {
		itemKey := keyOf(item)
		nextMap[itemKey] = item
		if _, ok := seen[itemKey]; !ok {
			keys = append(keys, itemKey)
			seen[itemKey] = struct{}{}
		}
	}
	slices.Sort(keys)

	for _, itemKey := range keys {
		currentItem, currentOK := currentMap[itemKey]
		nextItem, nextOK := nextMap[itemKey]

		switch {
		case !currentOK && nextOK:
			section.Added++
			section.Items = append(section.Items, updatePreviewDiffItem{
				Key:    itemKey,
				Title:  labelOf(nextItem),
				Detail: detailOf(nextItem),
				Change: "added",
			})
		case currentOK && !nextOK:
			section.Removed++
			section.Items = append(section.Items, updatePreviewDiffItem{
				Key:    itemKey,
				Title:  labelOf(currentItem),
				Detail: detailOf(currentItem),
				Change: "removed",
			})
		case stableJSON(currentItem) != stableJSON(nextItem):
			section.Changed++
			section.Items = append(section.Items, updatePreviewDiffItem{
				Key:    itemKey,
				Title:  labelOf(nextItem),
				Before: detailOf(currentItem),
				After:  detailOf(nextItem),
				Change: "changed",
			})
		default:
			section.Same++
		}
	}

	return section
}

func stableJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	normalized := normalizeJSON(decoded)
	normalizedRaw, err := json.Marshal(normalized)
	if err != nil {
		return string(raw)
	}
	return string(normalizedRaw)
}

func compactJSON(v any) string {
	raw, err := json.Marshal(normalizeJSON(v))
	if err != nil {
		return ""
	}
	return string(raw)
}

func normalizeJSON(v any) any {
	switch value := v.(type) {
	case []any:
		out := make([]any, len(value))
		allStrings := true
		stringVals := make([]string, len(value))
		for i, item := range value {
			normalized := normalizeJSON(item)
			out[i] = normalized
			asString, ok := normalized.(string)
			if !ok {
				allStrings = false
				continue
			}
			stringVals[i] = asString
		}
		if allStrings {
			slices.Sort(stringVals)
			sorted := make([]any, len(stringVals))
			for i, item := range stringVals {
				sorted[i] = item
			}
			return sorted
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[key] = normalizeJSON(item)
		}
		return out
	default:
		return value
	}
}

func sortedStringSetDiff(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for item := range a {
		if _, ok := b[item]; ok {
			continue
		}
		out = append(out, item)
	}
	slices.Sort(out)
	return out
}
