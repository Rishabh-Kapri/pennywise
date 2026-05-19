package model

import sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"

type StopReason = sharedModel.StopReason

const (
	StopReasonEndTurn   = sharedModel.StopReasonEndTurn
	StopReasonToolUse   = sharedModel.StopReasonToolUse
	StopReasonMaxTokens = sharedModel.StopReasonMaxTokens
	StopReasonError     = sharedModel.StopReasonError
)

type Usage = sharedModel.Usage
type ContentBlock = sharedModel.ContentBlock
type Message = sharedModel.AgentMessage
type ChatRequest = sharedModel.ChatRequest
type ChatResponse = sharedModel.ChatResponse
type ToolDefiniton = sharedModel.ToolDefiniton
type ToolSchema = sharedModel.ToolSchema
type ToolCall = sharedModel.ToolCall
type ToolResult = sharedModel.ToolResult
type ToolResultNormalized = sharedModel.ToolResultNormalized
type ToolChoice = sharedModel.ToolChoice

const (
	ToolChoiceAuto     = sharedModel.ToolChoiceAuto
	ToolChoiceNone     = sharedModel.ToolChoiceNone
	ToolChoiceRequired = sharedModel.ToolChoiceRequired
	ToolChoiceSpecific = sharedModel.ToolChoiceSpecific
)

type ChunkEvent = sharedModel.ChunkEvent

const (
	ChunkEventStarted       = sharedModel.ChunkEventStarted
	ChunkEventText          = sharedModel.ChunkEventText
	ChunkEventMessage       = sharedModel.ChunkEventMessage
	ChunkEventToolCallStart = sharedModel.ChunkEventToolCallStart
	ChunkEventToolCall      = sharedModel.ChunkEventToolCall
	ChunkEventToolCallDelta = sharedModel.ChunkEventToolCallDelta
	ChunkEventToolResult    = sharedModel.ChunkEventToolResult
	ChunkEventCompleted     = sharedModel.ChunkEventCompleted
	ChunkEventError         = sharedModel.ChunkEventError
)

type StreamChunk = sharedModel.StreamChunk
type StepResult = sharedModel.StepResult
type PubsubEvent = sharedModel.PubsubEvent
type IntentResult = sharedModel.IntentResult
type DateRange = sharedModel.DateRange
