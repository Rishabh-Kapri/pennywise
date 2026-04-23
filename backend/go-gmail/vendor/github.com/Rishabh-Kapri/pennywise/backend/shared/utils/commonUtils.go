package utils

import (
	"encoding/json"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// UnmarshalResponse unmarshals the given byte slice into a value of type T.
func UnmarshalResponse[T any](res []byte) (T, error) {
	var result T
	if err := json.Unmarshal(res, &result); err != nil {
		return result, errs.Wrap(errs.CodeInternalError, "error in unmarshalling", err)
	}
	return result, nil
}
