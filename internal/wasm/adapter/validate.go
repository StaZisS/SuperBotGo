package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func ValidateConfigAgainstSchema(schema json.RawMessage, config json.RawMessage) error {
	if len(schema) == 0 {
		return nil
	}

	schemaVal, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return fmt.Errorf("plugin config_schema is not valid JSON: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaVal); err != nil {
		return fmt.Errorf("plugin config_schema is invalid: %w", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("plugin config_schema compilation failed: %w", err)
	}

	configVal, err := jsonschema.UnmarshalJSON(bytes.NewReader(config))
	if err != nil {
		return fmt.Errorf("plugin config is not valid JSON: %w", err)
	}

	if err := compiled.Validate(configVal); err != nil {
		return formatValidationError(err)
	}
	return nil
}

func formatValidationError(err error) error {
	valErr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return fmt.Errorf("plugin config validation failed: %w", err)
	}

	details := collectValidationDetails(valErr)
	if len(details) == 0 {
		return fmt.Errorf("plugin config validation failed: %s", err.Error())
	}

	return fmt.Errorf("plugin config validation failed: %s", strings.Join(details, "; "))
}

func collectValidationDetails(ve *jsonschema.ValidationError) []string {
	if len(ve.Causes) == 0 {
		loc := fieldLocation(ve.InstanceLocation)
		msg := ve.ErrorKind.LocalizedString(message.NewPrinter(language.English))
		if loc == "" {
			return []string{msg}
		}
		return []string{fmt.Sprintf("field '%s': %s", loc, msg)}
	}
	var details []string
	for _, cause := range ve.Causes {
		details = append(details, collectValidationDetails(cause)...)
	}
	return details
}

func fieldLocation(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ".")
}
