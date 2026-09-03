package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

func decodeJSONObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple JSON values are not allowed")
		}
		return nil, err
	}
	document, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("the SARIF root must be a JSON object")
	}
	return document, nil
}

func cloneJSONObject(document map[string]any) (map[string]any, error) {
	data, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return decodeJSONObject(data)
}

func objectValue(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func arrayValue(value any) ([]any, bool) {
	result, ok := value.([]any)
	return result, ok
}

func stringValue(value any) (string, bool) {
	result, ok := value.(string)
	return result, ok
}

func firstString(values ...any) string {
	for _, value := range values {
		if result, ok := stringValue(value); ok && result != "" {
			return result
		}
	}
	return ""
}

func nestedValue(value any, keys ...string) any {
	current := value
	for _, key := range keys {
		object, ok := objectValue(current)
		if !ok {
			return nil
		}
		current = object[key]
	}
	return current
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	case float64:
		return int(typed), typed == float64(int(typed))
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func intOrZero(value any) int {
	result, _ := intValue(value)
	return result
}
