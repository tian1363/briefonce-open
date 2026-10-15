package reactagent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// validateToolInput implements the object, required, type and enum subset used by
// ScriptAgent's tool schemas. It rejects unknown fields before a handler runs.
func validateToolInput(rawSchema string, rawInput json.RawMessage) error {
	var input map[string]json.RawMessage
	if len(rawInput) == 0 || json.Unmarshal(rawInput, &input) != nil || input == nil {
		return errors.New("input must be a JSON object")
	}
	if strings.TrimSpace(rawSchema) == "" {
		return nil
	}
	var schema struct {
		Type       string `json:"type"`
		Properties map[string]struct {
			Type string   `json:"type"`
			Enum []string `json:"enum"`
		} `json:"properties"`
		Required             []string `json:"required"`
		AdditionalProperties bool     `json:"additionalProperties"`
	}
	if err := json.Unmarshal([]byte(rawSchema), &schema); err != nil || schema.Type != "object" {
		return errors.New("tool has invalid input schema")
	}
	for _, name := range schema.Required {
		value, ok := input[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) || bytes.Equal(bytes.TrimSpace(value), []byte(`""`)) {
			return fmt.Errorf("%s is required", name)
		}
	}
	for name, value := range input {
		field, ok := schema.Properties[name]
		if !ok {
			if !schema.AdditionalProperties {
				return fmt.Errorf("unknown field %s", name)
			}
			continue
		}
		switch field.Type {
		case "string":
			var text string
			if err := json.Unmarshal(value, &text); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("%s must be a string", name)
			}
			if len(field.Enum) > 0 {
				valid := false
				for _, allowed := range field.Enum {
					valid = valid || text == allowed
				}
				if !valid {
					return fmt.Errorf("%s has an unsupported value", name)
				}
			}
		case "integer":
			var number json.Number
			decoder := json.NewDecoder(bytes.NewReader(value))
			decoder.UseNumber()
			if err := decoder.Decode(&number); err != nil {
				return fmt.Errorf("%s must be an integer", name)
			}
			if _, err := number.Int64(); err != nil {
				return fmt.Errorf("%s must be an integer", name)
			}
		default:
			return fmt.Errorf("%s has an unsupported schema type", name)
		}
	}
	return nil
}
