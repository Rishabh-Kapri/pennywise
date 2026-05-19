package tools

import (
	"context"
	"encoding/json"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// Tool is the provider-neutral runtime contract for agent tools. Adapters only
// see Definition; the local runtime uses Execute after the LLM requests a call.
type Tool interface {
	// returns the tool definition
	Definition() sharedModel.ToolDefiniton
	// executes the tool
	Execute(ctx context.Context, call sharedModel.ToolCall) (*sharedModel.ToolResult, error)
	// get normalized name based on the status of tool call
	GetNormalizedName(isDone bool) string
	// return normalized result for ui
	Normalize(call sharedModel.ToolCall, result json.RawMessage) (*sharedModel.ToolResultNormalized, error)
}

func jsonToolResult(call sharedModel.ToolCall, name string, value any) (*sharedModel.ToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return &sharedModel.ToolResult{
		ToolCallId: call.ID,
		Name:       name,
		Content: []sharedModel.ContentBlock{
			{Type: "text", Text: string(data)},
		},
	}, nil
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) RegisterMultipleTools(tools []Tool) {
	for _, tool := range tools {
		r.RegisterTool(tool)
	}
}

func (r *ToolRegistry) RegisterTool(tool Tool) {
	logger.Logger(context.Background()).Info("registering tool", "tool", tool.Definition().Name)
	if tool == nil {
		return
	}

	toolName := tool.Definition().Name
	if toolName == "" {
		return
	}

	if _, exists := r.tools[toolName]; !exists {
		r.tools[toolName] = tool
	}
}

func (r *ToolRegistry) GetTool(name string) (Tool, error) {
	tool, ok := r.tools[name]
	if !ok || tool == nil {
		return nil, errs.New(errs.CodeToolNotFound, "no tool found for name: %s", name)
	}
	return tool, nil
}

func (r *ToolRegistry) GetAllTools() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		if tool == nil {
			continue
		}
		tools = append(tools, tool)
	}
	return tools
}
