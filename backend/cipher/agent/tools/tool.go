package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"

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

// decodeToolArgs unmarshals tool arguments, rejecting any field the target
// struct does not declare.
//
// encoding/json discards unknown keys by default, which is the wrong default
// here: a model that passes a filter the tool does not implement gets an
// unfiltered answer presented as a filtered one. Failing instead surfaces an
// IsError tool result, which the agent loop feeds back so the model can retry
// with arguments the tool actually supports.
func decodeToolArgs(name string, raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return errs.Wrap(errs.CodeInvalidArgument, "parse "+name+" arguments", err)
	}
	return nil
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

// ToolRegistry holds the agent's tools. Registration order is preserved because
// tools render first in the provider prompt: ranging a Go map shuffles the tool
// array on every request, which changes the prompt prefix and defeats prompt
// caching entirely.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string
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
	if tool == nil {
		return
	}

	toolName := tool.Definition().Name
	if toolName == "" {
		return
	}

	logger.Logger(context.Background()).Info("registering tool", "tool", toolName)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[toolName]; exists {
		return
	}
	r.tools[toolName] = tool
	r.order = append(r.order, toolName)
}

func (r *ToolRegistry) GetTool(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok || tool == nil {
		return nil, errs.New(errs.CodeToolNotFound, "no tool found for name: %s", name)
	}
	return tool, nil
}

// GetAllTools returns tools in registration order, so repeated requests produce
// a byte-identical tool array and the cached prompt prefix survives.
func (r *ToolRegistry) GetAllTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if tool, ok := r.tools[name]; ok && tool != nil {
			tools = append(tools, tool)
		}
	}
	return tools
}
