package utils

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
)

func CreateCursor(id string, createdAt time.Time, pointsNext bool) sharedModel.Cursor {
	parsedID, _ := uuid.Parse(id)
	return sharedModel.Cursor{
		ID:         parsedID,
		UpdatedAt:  createdAt,
		PointsNext: pointsNext,
	}
}

func GeneratePager(next sharedModel.Cursor, prev sharedModel.Cursor) sharedModel.Pagination {
	return sharedModel.Pagination{
		NextCursor: encodeCursor(next),
		PrevCursor: encodeCursor(prev),
	}
}

func GenerateCursorPager[T any](
	items []T,
	isFirstPage bool,
	pointsNext bool,
	hasMore bool,
	cursorFn func(T, bool) sharedModel.Cursor,
) sharedModel.Pagination {
	if len(items) == 0 {
		return sharedModel.Pagination{}
	}

	first := items[0]
	last := items[len(items)-1]
	var next sharedModel.Cursor
	var prev sharedModel.Cursor

	if !isFirstPage && (pointsNext || hasMore) {
		prev = cursorFn(first, false)
	}

	if hasMore || (!isFirstPage && !pointsNext) {
		next = cursorFn(last, true)
	}

	return GeneratePager(next, prev)
}

func CursorOperator(sortOrder string, pointsNext bool) string {
	if strings.EqualFold(sortOrder, "ASC") == pointsNext {
		return ">"
	}
	return "<"
}

func ReverseSortOrder(sortOrder string) string {
	if strings.EqualFold(sortOrder, "ASC") {
		return "DESC"
	}
	return "ASC"
}

func ReversePage[T any](items []T) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func encodeCursor(cursor sharedModel.Cursor) string {
	if cursor.ID == uuid.Nil && cursor.Date == "" && cursor.UpdatedAt.IsZero() {
		return ""
	}

	serialized, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(serialized)
	return string(encoded)
}

func DecodeCursor(cursor string) (sharedModel.Cursor, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return sharedModel.Cursor{}, err
	}

	var cur sharedModel.Cursor
	if err := json.Unmarshal(decoded, &cur); err != nil {
		return sharedModel.Cursor{}, err
	}

	return cur, nil
}

func CursorDateValue(cursor sharedModel.Cursor) (sharedModel.Date, error) {
	date := sharedModel.Date(cursor.Date)
	if err := date.Valid(); err != nil {
		return "", err
	}
	return date, nil
}

func CursorTime(cursor sharedModel.Cursor) (time.Time, error) {
	if cursor.UpdatedAt.IsZero() {
		return time.Time{}, errs.New(errs.CodeInvalidArgument, "invalid cursor")
	}

	return cursor.UpdatedAt, nil
}

func CursorUUID(cursor sharedModel.Cursor) (uuid.UUID, error) {
	if cursor.ID == uuid.Nil {
		return uuid.Nil, errs.New(errs.CodeInvalidArgument, "invalid cursor")
	}
	return cursor.ID, nil
}
