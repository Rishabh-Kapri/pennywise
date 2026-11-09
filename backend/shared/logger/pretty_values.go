package logger

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

var (
	errorType         = reflect.TypeOf((*error)(nil)).Elem()
	stringerType      = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

// Helpers that turn JSON strings, structs and maps into indented JSON for the
// console handler. Errors, Stringers and TextMarshalers keep their own text.

func prettyJSONString(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return "", false
	}

	var formatted bytes.Buffer
	if err := json.Indent(&formatted, []byte(trimmed), "", "  "); err != nil {
		return "", false
	}

	return formatted.String(), true
}

func prettyAny(value any) (string, bool) {
	if value == nil {
		return "", false
	}

	if raw, ok := value.(json.RawMessage); ok {
		return prettyJSONString(string(raw))
	}

	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return "", false
	}

	reflectedType := reflected.Type()
	if shouldSkipPrettyPrint(reflectedType) {
		return "", false
	}

	for reflectedType.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return "", false
		}
		reflected = reflected.Elem()
		reflectedType = reflected.Type()
		if shouldSkipPrettyPrint(reflectedType) {
			return "", false
		}
	}

	switch reflectedType.Kind() {
	case reflect.Struct, reflect.Map:
		return marshalPrettyJSON(reflected.Interface())
	default:
		return "", false
	}
}

func shouldSkipPrettyPrint(reflectedType reflect.Type) bool {
	return implementsInterface(reflectedType, errorType) ||
		implementsInterface(reflectedType, stringerType) ||
		implementsInterface(reflectedType, textMarshalerType)
}

func implementsInterface(reflectedType, target reflect.Type) bool {
	if reflectedType.Implements(target) {
		return true
	}
	if reflectedType.Kind() == reflect.Pointer {
		return false
	}

	return reflect.PointerTo(reflectedType).Implements(target)
}

func marshalPrettyJSON(value any) (string, bool) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw) == 0 {
		return "", false
	}
	if raw[0] != '{' && raw[0] != '[' {
		return "", false
	}

	return string(raw), true
}
