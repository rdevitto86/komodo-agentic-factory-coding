package line

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Schema is the JSON Schema subset the output device checks: type, required, enum, items, properties.
type Schema struct {
	Type       string            `json:"type"`
	Properties map[string]Schema `json:"properties"`
	Required   []string          `json:"required"`
	Items      *Schema           `json:"items"`
	Enum       []any             `json:"enum"`
}

// LoadSchema reads the schema a role returns.
func LoadSchema(root, role string) (Schema, error) {
	var schema Schema
	data, err := os.ReadFile(filepath.Join(root, RolesDir, role+".schema.json"))
	if err != nil {
		return schema, err
	}
	return schema, json.Unmarshal(data, &schema)
}

// Validate returns every way a result departs from its schema, deepest path first.
func Validate(schema Schema, value any) []string {
	return validateAt(schema, value, "result")
}

// validateAt checks one value against one schema node, naming the path it walked.
func validateAt(schema Schema, value any, path string) []string {
	var problems []string
	switch schema.Type {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: want an object, got %s", path, kind(value))}
		}
		for _, key := range schema.Required {
			if _, present := object[key]; !present {
				problems = append(problems, fmt.Sprintf("%s: missing required key %q", path, key))
			}
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, known := schema.Properties[key]
			if !known {
				continue
			}
			problems = append(problems, validateAt(child, object[key], path+"."+key)...)
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: want an array, got %s", path, kind(value))}
		}
		if schema.Items == nil {
			return problems
		}
		for index, item := range items {
			problems = append(problems, validateAt(*schema.Items, item, fmt.Sprintf("%s[%d]", path, index))...)
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return []string{fmt.Sprintf("%s: want a string, got %s", path, kind(value))}
		}
		if len(schema.Enum) > 0 && !inEnum(schema.Enum, text) {
			problems = append(problems, fmt.Sprintf("%s: %q is not one of %s", path, text, enumList(schema.Enum)))
		}
	case "integer", "number":
		if _, ok := value.(float64); !ok {
			return []string{fmt.Sprintf("%s: want a number, got %s", path, kind(value))}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return []string{fmt.Sprintf("%s: want a boolean, got %s", path, kind(value))}
		}
	}
	return problems
}

// inEnum reports whether a value is one the schema allows.
func inEnum(allowed []any, value string) bool {
	for _, item := range allowed {
		if text, ok := item.(string); ok && text == value {
			return true
		}
	}
	return false
}

// enumList renders an enum for a message.
func enumList(allowed []any) string {
	out := make([]string, 0, len(allowed))
	for _, item := range allowed {
		out = append(out, fmt.Sprint(item))
	}
	return strings.Join(out, ", ")
}

// kind names a JSON value's type for a message.
func kind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "a boolean"
	case float64:
		return "a number"
	case string:
		return "a string"
	case []any:
		return "an array"
	case map[string]any:
		return "an object"
	}
	return "something else"
}

// ReadResult reads a task's result JSON from disk.
func ReadResult(root, taskID string) (map[string]any, error) {
	data, path, err := ReadResultFile(root, taskID)
	if err != nil {
		return nil, err
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("%s is not JSON: %w", path, err)
	}
	return parsed, nil
}
