package utils

import (
	"encoding/json"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

func Marshal[T any](value T, truncLength int) ([]byte, error) {
	serialized, err := json.Marshal(value)
	if err != nil {
		return []byte{}, errs.Wrap(errs.CodeInternalError, "error in marshalling", err)
	}
	if truncLength != 0 && len(serialized) > truncLength {
		truncated := serialized[:truncLength]
		truncated = append(truncated, []byte("...truncated")...)
		return truncated, nil
	}
	return serialized, nil
}

// UnmarshalResponse unmarshals the given byte slice into a value of type T.
// Providers without a native JSON mode routinely wrap their reply in a ```json
// fence, so the fence is stripped first — this is a no-op for output that is
// already bare JSON.
func UnmarshalResponse[T any](res []byte) (T, error) {
	var result T
	if err := json.Unmarshal([]byte(StripMarkdownFence(string(res))), &result); err != nil {
		return result, errs.Wrap(errs.CodeInternalError, "error in unmarshalling", err)
	}
	return result, nil
}
