package logger

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

var (
	errorType         = reflect.TypeOf((*error)(nil)).Elem()
	stringerType      = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

type prettyHandler struct {
	next slog.Handler
}

func newPrettyHandler(next slog.Handler) slog.Handler {
	return &prettyHandler{next: next}
}

func (h *prettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	prettyRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		prettyRecord.AddAttrs(normalizeAttr(attr))
		return true
	})

	return h.next.Handle(ctx, prettyRecord)
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prettyHandler{next: h.next.WithAttrs(normalizeAttrs(attrs))}
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	return &prettyHandler{next: h.next.WithGroup(name)}
}

func normalizeAttrs(attrs []slog.Attr) []slog.Attr {
	normalized := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		normalized = append(normalized, normalizeAttr(attr))
	}

	return normalized
}

func normalizeAttr(attr slog.Attr) slog.Attr {
	attr.Key = prettifyKey(attr.Key)
	attr.Value = normalizeValue(attr.Value)
	return attr
}

func prettifyKey(key string) string {
	switch key {
	case "correlation_id":
		return "cid"
	case "budget_id", "budgetId", "X-Budget-ID":
		return "budget"
	case "user_id", "X-User-ID":
		return "user"
	default:
		return key
	}
}

func normalizeValue(value slog.Value) slog.Value {
	value = value.Resolve()

	switch value.Kind() {
	case slog.KindGroup:
		attrs := value.Group()
		normalized := make([]slog.Attr, 0, len(attrs))
		for _, attr := range attrs {
			normalized = append(normalized, normalizeAttr(attr))
		}
		return slog.GroupValue(normalized...)
	case slog.KindString:
		if pretty, ok := prettyJSONString(value.String()); ok {
			return slog.StringValue(pretty)
		}
	case slog.KindAny:
		if pretty, ok := prettyAny(value.Any()); ok {
			return slog.StringValue(pretty)
		}
	}

	return value
}

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
