package adapter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateConfigAgainstSchema_NoSchema(t *testing.T) {
	// When no schema is provided, validation should be skipped.
	err := ValidateConfigAgainstSchema(nil, json.RawMessage(`{"anything": true}`))
	if err != nil {
		t.Fatalf("expected nil error for nil schema, got: %v", err)
	}

	err = ValidateConfigAgainstSchema(json.RawMessage(``), json.RawMessage(`{"anything": true}`))
	if err != nil {
		t.Fatalf("expected nil error for empty schema, got: %v", err)
	}
}

func TestValidateConfigAgainstSchema_ValidConfig(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"api_key": {"type": "string"},
			"timeout": {"type": "number"}
		},
		"required": ["api_key"]
	}`)

	config := json.RawMessage(`{"api_key": "secret-123", "timeout": 30}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err != nil {
		t.Fatalf("expected valid config to pass, got: %v", err)
	}
}

func TestValidateConfigAgainstSchema_MissingRequired(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"api_key": {"type": "string"},
			"timeout": {"type": "number"}
		},
		"required": ["api_key"]
	}`)

	config := json.RawMessage(`{"timeout": 30}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for missing required field, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "plugin config validation failed") {
		t.Errorf("expected 'plugin config validation failed' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "api_key") {
		t.Errorf("expected 'api_key' mentioned in error, got: %s", errMsg)
	}
}

func TestValidateConfigAgainstSchema_WrongType(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"timeout": {"type": "number"}
		}
	}`)

	config := json.RawMessage(`{"timeout": "not-a-number"}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for wrong type, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "plugin config validation failed") {
		t.Errorf("expected 'plugin config validation failed' in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "timeout") {
		t.Errorf("expected 'timeout' mentioned in error, got: %s", errMsg)
	}
}

func TestValidateConfigAgainstSchema_NestedField(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"database": {
				"type": "object",
				"properties": {
					"port": {"type": "integer"}
				},
				"required": ["port"]
			}
		},
		"required": ["database"]
	}`)

	config := json.RawMessage(`{"database": {}}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for missing nested required field, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "port") {
		t.Errorf("expected 'port' mentioned in error, got: %s", errMsg)
	}
}

func TestValidateConfigAgainstSchema_InvalidSchema(t *testing.T) {
	schema := json.RawMessage(`{"type": "not-a-real-type"}`)
	config := json.RawMessage(`{}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for invalid schema, got nil")
	}
}

func TestValidateConfigAgainstSchema_InvalidConfigJSON(t *testing.T) {
	schema := json.RawMessage(`{"type": "object"}`)
	config := json.RawMessage(`{invalid json`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for invalid config JSON, got nil")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected 'not valid JSON' in error, got: %s", err.Error())
	}
}

func TestValidateConfigAgainstSchema_InvalidSchemaJSON(t *testing.T) {
	schema := json.RawMessage(`{invalid schema`)
	config := json.RawMessage(`{}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for invalid schema JSON, got nil")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected 'not valid JSON' in error, got: %s", err.Error())
	}
}

func TestValidateConfigAgainstSchema_MultipleErrors(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"api_key": {"type": "string"},
			"timeout": {"type": "number"},
			"retries": {"type": "integer"}
		},
		"required": ["api_key", "timeout"]
	}`)

	config := json.RawMessage(`{"retries": "not-an-int"}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	// Should mention missing required fields.
	if !strings.Contains(errMsg, "api_key") || !strings.Contains(errMsg, "timeout") {
		t.Errorf("expected both 'api_key' and 'timeout' in error, got: %s", errMsg)
	}
}

func TestValidateConfigAgainstSchema_EmptyConfigWithRequired(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"token": {"type": "string"}
		},
		"required": ["token"]
	}`)

	// Empty config (no JSON body at all).
	err := ValidateConfigAgainstSchema(schema, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for empty config with required fields, got nil")
	}
}

func TestValidateConfigAgainstSchema_AdditionalPropertiesFalse(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"additionalProperties": false
	}`)

	config := json.RawMessage(`{"name": "test", "unknown_field": 123}`)

	err := ValidateConfigAgainstSchema(schema, config)
	if err == nil {
		t.Fatal("expected error for additional property, got nil")
	}
	if !strings.Contains(err.Error(), "unknown_field") {
		t.Errorf("expected 'unknown_field' in error, got: %s", err.Error())
	}
}
