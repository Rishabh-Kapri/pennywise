package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

const (
	UpdateWorkingMemoryToolName = "update_working_memory"
)

type updateMemoryArgs struct {
	Operation string          `json:"operation"`
	Path      string          `json:"path"`
	Value     json.RawMessage `json:"value"`
}

type UpdateWorkingMemoryTool struct {
	memory repository.AgentMemoryRepository
}

func NewUpdateWorkingMemoryTool(memory repository.AgentMemoryRepository) Tool {
	return UpdateWorkingMemoryTool{memory: memory}
}

func (t UpdateWorkingMemoryTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        UpdateWorkingMemoryToolName,
		Description: "Store a lasting user preference or category/payee mapping. Only call when the user explicitly corrects your understanding or confirms a preference that should apply to future conversations.",
		InputSchema: sharedModel.ToolSchema{
			Type: "object",
			Properties: map[string]sharedModel.ToolSchema{
				"operation": {
					Type:        "string",
					Enum:        &[]any{"add", "remove", "replace"},
					Description: "add: append to an array. remove: delete from an array. replace: overwrite a scalar value.",
				},
				"path": {
					Type:        "string",
					Description: "Dot-separated path in the document. e.g. 'categoryAliases' or 'queryPreferences.medicalQueryStyle'",
				},
				"value": {
					Type:                 "object",
					Description:          "Non-empty JSON object to add, remove, or replace at the given path. Example for queryPreferences.medicalQueryStyle: {\"includePayeeAliases\": true, \"includeCategoryAliases\": true, \"notes\": \"Treat medical spending as both medical categories and confirmed medical payees.\"}",
					AdditionalProperties: true,
				},
			},
			Required:             []string{"operation", "path", "value"},
			AdditionalProperties: false,
		},
	}
}

func (t UpdateWorkingMemoryTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	var args updateMemoryArgs

	if err := decodeToolArgs(UpdateWorkingMemoryToolName, call.Arguments, &args); err != nil {
		return nil, err
	}
	valueJSON := args.Value
	if len(valueJSON) == 0 || strings.TrimSpace(string(valueJSON)) == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "update_working_memory value is required")
	}
	if strings.TrimSpace(string(valueJSON)) == "{}" {
		return nil, errs.New(errs.CodeInvalidArgument, "update_working_memory value must not be an empty object")
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "update_working_memory path is required")
	}
	path := strings.Split(args.Path, ".")

	// The budget comes from the request context, never from tool arguments: a
	// model-supplied budget id here would let a hallucination or a prompt
	// injection write into another tenant's memory.
	err := t.memory.PatchWorkingMemory(ctx, utils.MustBudgetID(ctx), repository.WorkingMemoryOp(args.Operation), path, valueJSON)
	if errors.Is(err, repository.ErrUnknownWorkingMemoryOp) {
		return nil, errs.New(errs.CodeInternalError, "wrong operation")
	}
	if err != nil {
		return nil, err
	}
	return jsonToolResult(call, call.Name, "updated memory")
}

func (t UpdateWorkingMemoryTool) GetNormalizedName(isDone bool) string {
	return ""
}

func (t UpdateWorkingMemoryTool) Normalize(
	call sharedModel.ToolCall,
	result json.RawMessage,
) (*sharedModel.ToolResultNormalized, error) {
	return nil, nil
}
