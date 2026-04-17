package api

import (
	"encoding/json"
	"strings"
	"testing"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func TestBuildUpdatePreviewResponse(t *testing.T) {
	t.Parallel()

	current := wasmrt.PluginMeta{
		ID:      "demo",
		Name:    "Demo",
		Version: "1.2.0",
		RPCMethods: []wasmrt.RPCMethodDef{
			{Name: "profile.get", Description: "Read profile"},
		},
		Triggers: []wasmrt.TriggerDef{
			{Name: "ping", Type: "messenger", Description: "Ping command"},
		},
		Requirements: []wasmrt.RequirementDef{
			{Type: "http", Name: "default", Description: "External API"},
		},
		ConfigSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"token":{"type":"string","description":"API token"}
			},
			"required":["token"]
		}`),
	}

	next := wasmrt.PluginMeta{
		ID:      "demo",
		Name:    "Demo",
		Version: "1.3.0",
		RPCMethods: []wasmrt.RPCMethodDef{
			{Name: "profile.get", Description: "Read profile v2"},
			{Name: "profile.sync", Description: "Sync profile"},
		},
		Triggers: []wasmrt.TriggerDef{
			{Name: "ping", Type: "messenger", Description: "Ping command updated"},
			{Name: "sync", Type: "http", Methods: []string{"POST"}, Path: "/sync", Description: "Sync endpoint"},
		},
		Requirements: []wasmrt.RequirementDef{
			{Type: "http", Name: "default", Description: "External API", Config: json.RawMessage(`{"allowed_hosts":["api.example.com"]}`)},
			{Type: "plugin", Target: "audit", Description: "Audit trail"},
		},
		ConfigSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"token":{"type":"string","description":"API token for upstream"},
				"timeout":{"type":"integer","description":"Timeout in seconds"}
			},
			"required":["token","timeout"]
		}`),
	}

	preview := buildUpdatePreviewResponse(current, next)

	if !preview.CanUpdate {
		t.Fatal("CanUpdate = false, want true")
	}
	if !preview.HasChanges {
		t.Fatal("HasChanges = false, want true")
	}
	if preview.Current.Version != "1.2.0" || preview.Next.Version != "1.3.0" {
		t.Fatalf("unexpected versions in preview: current=%q next=%q", preview.Current.Version, preview.Next.Version)
	}
	if len(preview.Warnings) != 0 {
		t.Fatalf("Warnings = %v, want none for upgrade preview", preview.Warnings)
	}

	triggerSection := findPreviewSection(t, preview.Sections, "triggers")
	if triggerSection.Added != 1 || triggerSection.Changed != 1 {
		t.Fatalf("trigger diff counts = added:%d changed:%d, want added:1 changed:1", triggerSection.Added, triggerSection.Changed)
	}

	requirementSection := findPreviewSection(t, preview.Sections, "requirements")
	if requirementSection.Added != 1 || requirementSection.Changed != 1 {
		t.Fatalf("requirement diff counts = added:%d changed:%d, want added:1 changed:1", requirementSection.Added, requirementSection.Changed)
	}

	rpcSection := findPreviewSection(t, preview.Sections, "rpc_methods")
	if rpcSection.Added != 1 || rpcSection.Changed != 1 {
		t.Fatalf("rpc diff counts = added:%d changed:%d, want added:1 changed:1", rpcSection.Added, rpcSection.Changed)
	}

	schemaSection := findPreviewSection(t, preview.Sections, "config_schema")
	if schemaSection.Added != 1 || schemaSection.Changed != 2 {
		t.Fatalf("schema diff counts = added:%d changed:%d, want added:1 changed:2", schemaSection.Added, schemaSection.Changed)
	}

	versionSummary := findPreviewSummary(t, preview.Summary, "version")
	if !versionSummary.Changed || versionSummary.Current != "1.2.0" || versionSummary.Next != "1.3.0" {
		t.Fatalf("unexpected version summary: %+v", versionSummary)
	}
}

func TestBuildUpdatePreviewResponseBlocksMismatchedPluginID(t *testing.T) {
	t.Parallel()

	current := wasmrt.PluginMeta{ID: "demo", Name: "Demo", Version: "1.0.0"}
	next := wasmrt.PluginMeta{ID: "other", Name: "Other", Version: "2.0.0"}

	preview := buildUpdatePreviewResponse(current, next)

	if preview.CanUpdate {
		t.Fatal("CanUpdate = true, want false for plugin id mismatch")
	}
	if !hasPreviewWarning(preview.Warnings, "plugin_id_mismatch") {
		t.Fatalf("expected plugin_id_mismatch warning, got %+v", preview.Warnings)
	}
}

func TestBuildUpdatePreviewResponseIncludesEmptyWarningsArray(t *testing.T) {
	t.Parallel()

	preview := buildUpdatePreviewResponse(
		wasmrt.PluginMeta{ID: "demo", Name: "Demo", Version: "1.0.0"},
		wasmrt.PluginMeta{ID: "demo", Name: "Demo", Version: "1.1.0"},
	)

	raw, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("json.Marshal(preview) error = %v", err)
	}

	if !strings.Contains(string(raw), `"warnings":[]`) {
		t.Fatalf("warnings field missing or not empty array: %s", string(raw))
	}
}

func findPreviewSection(t *testing.T, sections []updatePreviewSection, key string) updatePreviewSection {
	t.Helper()
	for _, section := range sections {
		if section.Key == key {
			return section
		}
	}
	t.Fatalf("section %q not found", key)
	return updatePreviewSection{}
}

func findPreviewSummary(t *testing.T, summaries []updatePreviewSummary, key string) updatePreviewSummary {
	t.Helper()
	for _, summary := range summaries {
		if summary.Key == key {
			return summary
		}
	}
	t.Fatalf("summary %q not found", key)
	return updatePreviewSummary{}
}

func hasPreviewWarning(warnings []updatePreviewWarning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
