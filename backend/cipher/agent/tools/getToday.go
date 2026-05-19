package tools

import (
	"context"
	"encoding/json"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

const getTodayToolName = "get_today"

type GetTodayTool struct {
	Location *time.Location
}

func NewGetTodayTool(location *time.Location) Tool {
	if location == nil {
		location = time.Local
	}

	return GetTodayTool{Location: location}
}

func (t GetTodayTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        getTodayToolName,
		Description: "Return today's date in the user's timezone. Use this for date arithmetic instead of guessing the current date.",
		InputSchema: sharedModel.ToolSchema{
			Type:                 "object",
			Properties:           map[string]sharedModel.ToolSchema{},
			Required:             []string{},
			AdditionalProperties: false,
		},
	}
}

func (t GetTodayTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	now := time.Now().In(t.Location)

	return jsonToolResult(call, getTodayToolName, map[string]string{
		"date":     now.Format(time.DateOnly),
		"timezone": t.Location.String(),
	})
}

func (t GetTodayTool) GetNormalizedName(isDone bool) string {
	if (isDone) {
		return "Fetched date"
	}
	return "Fetching date..."
}

func (t GetTodayTool) Normalize(call sharedModel.ToolCall, result json.RawMessage) (*sharedModel.ToolResultNormalized, error) {
	// its fine to result the result of this tool as it is
	return &sharedModel.ToolResultNormalized{
		DisplayName: t.GetNormalizedName(true),
		Summary:     "",
		Result:      result,
	}, nil
}
