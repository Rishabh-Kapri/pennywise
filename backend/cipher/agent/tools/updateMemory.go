package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	updateWorkingMemoryToolName = "update_working_memory"
)

type updateMemoryArgs struct {
	Operation string          `json:"operation"`
	Path      string          `json:"path"`
	Value     json.RawMessage `json:"value"`
	BudgetID  string          `json:"budgetId"`
}

type UpdateWorkingMemoryTool struct {
	db.BaseRepository
}

func NewUpdateWorkingMemoryTool(pool *pgxpool.Pool) Tool {
	return UpdateWorkingMemoryTool{BaseRepository: db.NewBaseRepository(pool)}
}

func (t UpdateWorkingMemoryTool) Definition() sharedModel.ToolDefiniton {
	return sharedModel.ToolDefiniton{
		Name:        updateWorkingMemoryToolName,
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
				"budgetId": {
					Type:        "string",
					Description: "The budgetId to query the working memory for.",
				},
			},
			Required:             []string{"operation", "path", "value", "budgetId"},
			AdditionalProperties: false,
		},
	}
}

func (t UpdateWorkingMemoryTool) Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error) {
	var query string
	var args updateMemoryArgs

	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parse update_working_memory arguments", err)
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
	budgetIDParsed, err := uuid.Parse(args.BudgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "parsing budgetId", err)
	}
	queryArgs := []any{path, string(valueJSON), budgetIDParsed}

	switch args.Operation {
	case "add":
		query = `
		  INSERT INTO working_memory(budget_id, document)
		  VALUES ($3, jsonb_set('{}', $1::text[], $2::jsonb, true))
		  ON CONFLICT (budget_id) DO UPDATE
		  SET document = jsonb_set(
		    working_memory.document,
		    $1::text[],
			  (COALESCE(working_memory.document #> $1::text[], '[]'::jsonb) || $2::jsonb),
			  true
			),
			updated_at = now()
			`

	case "replace":
		query = `
		  INSERT INTO working_memory(budget_id, document)
      VALUES ($3, jsonb_set('{}', $1::text[], $2::jsonb, true))
		  ON CONFLICT (budget_id) DO UPDATE
		  SET document = jsonb_set(
		    working_memory.document,
		    $1::text[],
			  $2::jsonb,
			  true
			),
			updated_at = now()
			`
	case "remove":
		query = `
		  INSERT INTO working_memory(budget_id, document)
      VALUES ($3, '{}'::jsonb)
		  ON CONFLICT (budget_id) DO UPDATE
			SET document = jsonb_set(
				working_memory.document,
				$1::text[],
				COALESCE((
					SELECT jsonb_agg(elem)
          FROM jsonb_array_elements(working_memory.document #> $1::text[]) elem
					WHERE elem != $2::jsonb
				), '[]'::jsonb),
				true
			),
			updated_at = now()
		`

	default:
		return nil, errs.New(errs.CodeInternalError, "wrong operation")
	}
	_, err = t.Executor(nil).Exec(ctx, query, queryArgs...)
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
