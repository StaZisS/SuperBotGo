package wasmplugin

import "encoding/json"

// ConfigSchema is a type-safe JSON Schema built via ConfigFields / Field helpers.
type ConfigSchema struct {
	fields   []fieldDef
	required []string
}

type fieldDef struct {
	key  string
	prop schemaProperty
}

type schemaProperty struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Minimum     *float64    `json:"minimum,omitempty"`
	Maximum     *float64    `json:"maximum,omitempty"`
	MinLength   *int        `json:"minLength,omitempty"`
	MaxLength   *int        `json:"maxLength,omitempty"`
	Pattern     string      `json:"pattern,omitempty"`
	Sensitive   bool        `json:"-"` // hint for UI: render as password
}

// MarshalJSON produces a valid JSON Schema object.
func (s ConfigSchema) MarshalJSON() ([]byte, error) {
	props := make(map[string]interface{}, len(s.fields))
	for _, f := range s.fields {
		p := map[string]interface{}{"type": f.prop.Type}
		if f.prop.Description != "" {
			p["description"] = f.prop.Description
		}
		if f.prop.Default != nil {
			p["default"] = f.prop.Default
		}
		if len(f.prop.Enum) > 0 {
			p["enum"] = f.prop.Enum
		}
		if f.prop.Minimum != nil {
			p["minimum"] = *f.prop.Minimum
		}
		if f.prop.Maximum != nil {
			p["maximum"] = *f.prop.Maximum
		}
		if f.prop.MinLength != nil {
			p["minLength"] = *f.prop.MinLength
		}
		if f.prop.MaxLength != nil {
			p["maxLength"] = *f.prop.MaxLength
		}
		if f.prop.Pattern != "" {
			p["pattern"] = f.prop.Pattern
		}
		props[f.key] = p
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(s.required) > 0 {
		schema["required"] = s.required
	}
	return json.Marshal(schema)
}

// IsEmpty returns true if no fields are defined.
func (s ConfigSchema) IsEmpty() bool {
	return len(s.fields) == 0
}

// ConfigFields creates a ConfigSchema from field definitions.
func ConfigFields(fields ...Field) ConfigSchema {
	s := ConfigSchema{}
	for _, f := range fields {
		s.fields = append(s.fields, fieldDef{key: f.key, prop: f.prop})
		if f.required {
			s.required = append(s.required, f.key)
		}
	}
	return s
}

// Field is a config field builder. Create via String, Number, Integer, Bool, or Enum.
type Field struct {
	key      string
	prop     schemaProperty
	required bool
}

// --- Constructors ---

// String creates a string config field.
func String(key, description string) Field {
	return Field{key: key, prop: schemaProperty{Type: "string", Description: description}}
}

// Number creates a number config field.
func Number(key, description string) Field {
	return Field{key: key, prop: schemaProperty{Type: "number", Description: description}}
}

// Integer creates an integer config field.
func Integer(key, description string) Field {
	return Field{key: key, prop: schemaProperty{Type: "integer", Description: description}}
}

// Bool creates a boolean config field.
func Bool(key, description string) Field {
	return Field{key: key, prop: schemaProperty{Type: "boolean", Description: description}}
}

// Enum creates a string field with a fixed set of allowed values.
func Enum(key, description string, values ...string) Field {
	return Field{key: key, prop: schemaProperty{Type: "string", Description: description, Enum: values}}
}

// --- Modifiers (chainable) ---

// Default sets a default value.
func (f Field) Default(v interface{}) Field {
	f.prop.Default = v
	return f
}

// Required marks the field as required.
func (f Field) Required() Field {
	f.required = true
	return f
}

// Min sets the minimum value (for Number/Integer).
func (f Field) Min(n float64) Field {
	f.prop.Minimum = &n
	return f
}

// Max sets the maximum value (for Number/Integer).
func (f Field) Max(n float64) Field {
	f.prop.Maximum = &n
	return f
}

// MinLen sets the minimum string length.
func (f Field) MinLen(n int) Field {
	f.prop.MinLength = &n
	return f
}

// MaxLen sets the maximum string length.
func (f Field) MaxLen(n int) Field {
	f.prop.MaxLength = &n
	return f
}

// Pattern sets a regex pattern for validation.
func (f Field) Pattern(re string) Field {
	f.prop.Pattern = re
	return f
}

// Sensitive marks the field for UI to render as a password input.
// Appends "_sensitive" hint to the key name convention.
func (f Field) Sensitive() Field {
	f.prop.Sensitive = true
	return f
}
