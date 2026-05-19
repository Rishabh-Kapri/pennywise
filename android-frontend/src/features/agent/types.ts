import type { LoadingState } from '../../utils/constants';

export type AgentRunStatus = 'QUEUED' | 'RUNNING' | 'COMPLETED' | 'FAILED' | 'CANCELLED';

export type ConversationRole = 'USER' | 'ASSISTANT' | 'TOOL' | 'SYSTEM';

export type AgentEventMessageData = {
  id?: string;
  type: string;
  message: unknown;
};

export type MessagePart = {
  type: string;
  content?: string;
  id?: string;
  name?: string;
  displayName?: string;
  summary?: string;
  args?: Record<string, unknown>;
  result?: unknown;
};

export type MessageContent = {
  id?: string;
  role?: ConversationRole | Lowercase<ConversationRole>;
  parts?: MessagePart[];
  createdAt?: string;
  created_at?: string;
};

export type ConversationMessageContent = MessagePart[] | MessageContent | MessageContent[];

export type AgentConversation = {
  id: string;
  agentKey: string;
  userId: string;
  budgetId: string;
  title?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type ConversationMessage = {
  id: string;
  conversationId: string;
  runId?: string;
  sequence: number;
  content?: ConversationMessageContent | string;
  role?: ConversationRole;
  text?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
};

export type AgentRunCreateRequest = {
  message: string;
  budgetId?: string;
  runId?: string;
  agentKey?: string;
  conversationId?: string;
  title?: string;
  modelProvider?: string;
  modelName?: string;
  temperature?: number;
  maxTokens?: number;
  stream?: boolean;
  conversationMetadata?: Record<string, unknown>;
  messageMetadata?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type AgentRun = {
  id: string;
  agentKey: string;
  userId?: string;
  budgetId?: string;
  conversationId?: string;
  streamId?: string;
  status: AgentRunStatus;
  modelProvider?: string;
  modelName?: string;
  temperature?: number;
  maxTokens?: number;
  userMessage?: string;
  finalMessage?: string;
  error?: string;
  traceId?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt?: string;
  updatedAt?: string;
  metadata?: Record<string, unknown>;
  conversation?: AgentConversation;
  messages?: ConversationMessage[];
};

export type AgentChatMessage = {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  parts?: MessagePart[];
  eventName?: string;
  runId?: string;
  streamMessageId?: string;
};

export type AgentChatHistoryItem = {
  id: string;
  title: string;
  messages: AgentChatMessage[];
  metadata?: Record<string, unknown>;
  streamId?: string;
  updatedAt?: string;
};

export type AgentModelOption = {
  key: string;
  label: string;
  shortLabel?: string;
  provider: string;
  modelName: string;
};

export type AgentState = {
  messages: AgentChatMessage[];
  chatHistoryById: Record<string, AgentChatHistoryItem>;
  chatHistoryOrder: string[];
  runsById: Record<string, AgentRun>;
  currentRunId: string | null;
  currentConversationId: string | null;
  currentStreamId: string | null;
  selectedModelKey: string;
  createRunLoading: LoadingState;
  error: string | null;
  nextMessageId: number;
};
