package repository

import "errors"

// ErrNotFound is returned (wrapped) by repositories when the requested row
// does not exist. Check it with errors.Is.
var ErrNotFound = errors.New("not found")
