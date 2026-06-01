import { createAsyncThunk, createSlice, type PayloadAction } from '@reduxjs/toolkit';
import type { RootState } from '../../../app/store';
import { LoadingState } from '../../../utils/constants';
import { apiClient } from '../../../utils/api';
import type {
  AgentChatHistoryItem,
  AgentChatMessage,
  AgentConversation,
  AgentModelOption,
  AgentRun,
  AgentRunCreateRequest,
  AgentState,
  ConversationMessage,
  MessageContent,
  MessagePart
} from '../types';

const TEXT_DELTA_EVENT = 'agent::chat::text_delta';

export const AGENT_MODEL_OPTIONS: AgentModelOption[] = [
  {
    key: 'sonnet-4-6',
    label: 'Claude Sonnet 4.6',
    shortLabel: 'Sonnet 4.6',
    provider: 'anthropic',
    modelName: 'claude-sonnet-4-6'
  },
  {
    key: 'gpt-5.4',
    label: 'GPT 5.4',
    shortLabel: 'GPT 5.4',
    provider: 'openai',
    modelName: 'gpt-5.4'
  },
  {
    key: 'gpt-5.4-mini',
    label: 'GPT 5.4 Mini',
    shortLabel: 'GPT Mini',
    provider: 'openai',
    modelName: 'gpt-5.4-mini'
  },
  {
    key: 'openrouter-haiku-4-5',
    label: 'OpenRouter Claude Haiku 4.5',
    shortLabel: 'OR Haiku 4.5',
    provider: 'openrouter',
    modelName: 'anthropic/claude-haiku-4.5'
  },
  {
    key: 'openrouter-sonnet-4-5',
    label: 'OpenRouter Claude Sonnet 4.5',
    shortLabel: 'OR Sonnet 4.5',
    provider: 'openrouter',
    modelName: 'anthropic/claude-sonnet-4.5'
  },
  {
    key: 'haiku-4-5',
    label: 'Claude Haiku 4.5',
    shortLabel: 'Haiku 4.5',
    provider: 'anthropic',
    modelName: 'claude-haiku-4-5'
  }
];

const DEFAULT_AGENT_MODEL_KEY = 'sonnet-4-6';

const initialState: AgentState = {
  messages: [],
  chatHistoryById: {},
  chatHistoryOrder: [],
  runsById: {},
  currentRunId: null,
  currentConversationId: null,
  currentStreamId: null,
  selectedModelKey: DEFAULT_AGENT_MODEL_KEY,
  createRunLoading: LoadingState.IDLE,
  error: null,
  nextMessageId: 0
};

type CreateAgentRunArgs = Pick<
  AgentRunCreateRequest,
  'message' | 'conversationId' | 'title' | 'modelProvider' | 'modelName' | 'temperature' | 'maxTokens' | 'metadata'
>;

type AppendAgentEventPayload = {
  eventName: string;
  text: string;
};

type AppendAgentTextDeltaPayload = {
  text: string;
  messageId?: string;
};

type AppendAgentMessagePartPayload = {
  part: MessagePart;
  eventName?: string;
  messageId?: string;
};

type ConversationMessagesResponse = ConversationMessage[] | { messages?: ConversationMessage[] };

function nextMessageID(state: AgentState, prefix: string) {
  state.nextMessageId += 1;
  return `${prefix}-${state.nextMessageId}`;
}

function appendAssistantMessage(state: AgentState, message: Omit<AgentChatMessage, 'id' | 'role'>) {
  state.messages.push({
    id: nextMessageID(state, 'agent-message'),
    role: 'assistant',
    ...message
  });
}

function cloneChatMessages(messages: AgentChatMessage[]) {
  return messages.map((message) => ({
    ...message,
    parts: message.parts?.map((part) => ({ ...part }))
  }));
}

function chatTitleFromMessages(messages: AgentChatMessage[]) {
  const firstUserMessage = messages.find((message) => message.role === 'user')?.text.trim();
  if (!firstUserMessage) {
    return 'New chat';
  }

  return firstUserMessage.length > 42 ? `${firstUserMessage.slice(0, 39)}...` : firstUserMessage;
}

function chatTitleFromConversation(conversation: AgentConversation) {
  return conversation.title?.trim();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function isMessagePart(value: unknown): value is MessagePart {
  return isRecord(value) && typeof value.type === 'string';
}

function isMessageContent(value: unknown): value is MessageContent {
  return isRecord(value) && Array.isArray(value.parts);
}

function normalizeMessageParts(content: ConversationMessage['content']): MessagePart[] {
  if (!content) {
    return [];
  }

  if (typeof content === 'string') {
    try {
      return normalizeMessageParts(JSON.parse(content) as ConversationMessage['content']);
    } catch {
      return [{ type: 'TEXT', content }];
    }
  }

  if (Array.isArray(content)) {
    return content.flatMap((item) => {
      if (isMessagePart(item)) {
        return [item];
      }
      if (isMessageContent(item)) {
        return item.parts ?? [];
      }
      return [];
    });
  }

  if (isMessagePart(content)) {
    return [content];
  }
  if (isMessageContent(content)) {
    return content.parts ?? [];
  }
  return [];
}

function chatRoleFromConversationRole(role?: string) {
  return role?.toUpperCase() === 'USER' ? 'user' : 'assistant';
}

function textFromMessageParts(parts: MessagePart[]) {
  return parts
    .map((part) => {
      const partType = part.type.toUpperCase();
      if (partType !== 'TEXT') {
        return '';
      }
      if (typeof part.content === 'string') {
        return part.content;
      }
      if (typeof part.result === 'string') {
        return part.result;
      }
      return '';
    })
    .filter(Boolean)
    .join('\n\n');
}

function mergeMessageParts(existingParts: MessagePart[] = [], newParts: MessagePart[] = []) {
  const mergedParts = [...existingParts];
  for (const part of newParts) {
    const existingIndex = part.id ? mergedParts.findIndex((existingPart) => existingPart.id === part.id) : -1;
    if (existingIndex >= 0) {
      mergedParts[existingIndex] = part;
    } else {
      mergedParts.push(part);
    }
  }
  return mergedParts;
}

function mergeMessageText(existingText: string, newText: string) {
  if (!newText.trim()) {
    return existingText;
  }
  if (!existingText.trim()) {
    return newText;
  }
  return `${existingText}\n\n${newText}`;
}

function chatMessagesFromConversationMessages(messages: ConversationMessage[]) {
  const groupedMessages = new Map<string, AgentChatMessage>();

  for (const message of messages) {
    const parts = normalizeMessageParts(message.content);
    const text = parts.length > 0 ? textFromMessageParts(parts) : message.text ?? '';
    const existingMessage = groupedMessages.get(message.id);

    if (existingMessage) {
      existingMessage.text = mergeMessageText(existingMessage.text, text);
      existingMessage.parts = mergeMessageParts(existingMessage.parts, parts);
      existingMessage.runId = existingMessage.runId ?? message.runId;
      existingMessage.streamMessageId = existingMessage.streamMessageId ?? message.id;
      continue;
    }

    groupedMessages.set(message.id, {
      id: message.id,
      role: chatRoleFromConversationRole(message.role),
      text,
      parts: parts.length > 0 ? parts : undefined,
      runId: message.runId,
      streamMessageId: message.id
    });
  }

  return Array.from(groupedMessages.values()).filter(
    (chatMessage) => chatMessage.text.trim() !== '' || (chatMessage.parts?.length ?? 0) > 0
  );
}

function normalizeConversationMessagesResponse(response: ConversationMessagesResponse) {
  return Array.isArray(response) ? response : response.messages ?? [];
}

function moveConversationToFront(state: AgentState, conversationId: string) {
  state.chatHistoryOrder = [conversationId, ...state.chatHistoryOrder.filter((id) => id !== conversationId)];
}

function upsertChatHistory(
  state: AgentState,
  conversationId: string,
  overrides: Partial<Pick<AgentChatHistoryItem, 'title' | 'metadata' | 'streamId' | 'updatedAt'>> = {}
) {
  const existing = state.chatHistoryById[conversationId];
  const title = overrides.title?.trim() || existing?.title || chatTitleFromMessages(state.messages);

  state.chatHistoryById[conversationId] = {
    id: conversationId,
    title,
    messages: cloneChatMessages(state.messages),
    metadata: overrides.metadata ?? existing?.metadata,
    streamId: overrides.streamId ?? existing?.streamId ?? state.currentStreamId ?? undefined,
    updatedAt: overrides.updatedAt ?? existing?.updatedAt
  };
  moveConversationToFront(state, conversationId);
}

function mergeConversationHistory(state: AgentState, conversations: AgentConversation[]) {
  syncCurrentChatHistory(state);

  const listedConversationIds = new Set<string>();
  const listedOrder: string[] = [];

  for (const conversation of conversations) {
    const existing = state.chatHistoryById[conversation.id];
    listedConversationIds.add(conversation.id);
    listedOrder.push(conversation.id);

    state.chatHistoryById[conversation.id] = {
      id: conversation.id,
      title: chatTitleFromConversation(conversation) || existing?.title || 'New chat',
      messages: existing ? cloneChatMessages(existing.messages) : [],
      metadata: conversation.metadata ?? existing?.metadata,
      streamId: existing?.streamId ?? conversation.id,
      updatedAt: conversation.updatedAt
    };
  }

  if (
    state.currentConversationId &&
    !listedConversationIds.has(state.currentConversationId) &&
    (state.chatHistoryById[state.currentConversationId]?.messages.length ?? 0) > 0
  ) {
    listedOrder.unshift(state.currentConversationId);
  }

  state.chatHistoryOrder = listedOrder;
}

function syncCurrentChatHistory(state: AgentState) {
  if (!state.currentConversationId) {
    return;
  }

  upsertChatHistory(state, state.currentConversationId);
}

function lastRunId(messages: AgentChatMessage[]) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const runId = messages[index]?.runId;
    if (runId) {
      return runId;
    }
  }

  return null;
}

function findAssistantMessageForStream(state: AgentState, messageId?: string) {
  if (messageId) {
    return state.messages.find(
      (message) => message.role === 'assistant' && (message.streamMessageId === messageId || message.id === messageId)
    );
  }

  const lastMessage = state.messages[state.messages.length - 1];
  return lastMessage?.role === 'assistant' ? lastMessage : undefined;
}

function resolveAgentRunStreamId(run: AgentRun) {
  const returnedStreamId = run.streamId ?? (run as { streamID?: string }).streamID;
  if (returnedStreamId) {
    return returnedStreamId;
  }

  if (run.conversationId) {
    return run.conversationId;
  }

  return null;
}

export const createAgentRun = createAsyncThunk<AgentRun, CreateAgentRunArgs, { state: RootState }>(
  'agent/createRun',
  async (args, { getState }) => {
    const state = getState();
    const conversationId = args.conversationId ?? state.agent.currentConversationId ?? undefined;
    const selectedModel = AGENT_MODEL_OPTIONS.find((option) => option.key === state.agent.selectedModelKey) ?? AGENT_MODEL_OPTIONS[0];

    const payload: AgentRunCreateRequest = {
      ...args,
      modelProvider: args.modelProvider ?? selectedModel.provider,
      modelName: args.modelName ?? selectedModel.modelName,
      stream: true
    };
    if (conversationId) {
      payload.conversationId = conversationId;
    }

    return apiClient.post<AgentRun>('agent/runs', payload);
  }
);

export const listAgentConversations = createAsyncThunk<AgentConversation[]>('agent/listConversations', async () => {
  return apiClient.get<AgentConversation[]>('agent/conversations');
});

export const fetchAgentConversationMessages = createAsyncThunk<{ conversationId: string; messages: ConversationMessage[] }, string>(
  'agent/fetchConversationMessages',
  async (conversationId) => {
    const response = await apiClient.get<ConversationMessagesResponse>(`agent/conversations/${conversationId}/messages`);
    return {
      conversationId,
      messages: normalizeConversationMessagesResponse(response)
    };
  }
);

export const deleteAgentConversation = createAsyncThunk<string, string>('agent/deleteConversation', async (conversationId) => {
  await apiClient.delete<{ message: string }>(`agent/conversations/${conversationId}`);
  return conversationId;
});

const agentSlice = createSlice({
  name: 'agent',
  initialState,
  reducers: {
    appendAgentTextDelta: (state, action: PayloadAction<AppendAgentTextDeltaPayload>) => {
      const text = action.payload.text;
      if (!text) {
        return;
      }

      const targetMessage = findAssistantMessageForStream(state, action.payload.messageId);
      if (targetMessage) {
        targetMessage.text += text;
        targetMessage.eventName = targetMessage.eventName ?? TEXT_DELTA_EVENT;
        targetMessage.streamMessageId = action.payload.messageId ?? targetMessage.streamMessageId;
        syncCurrentChatHistory(state);
        return;
      }

      appendAssistantMessage(state, {
        eventName: TEXT_DELTA_EVENT,
        text,
        runId: state.currentRunId ?? undefined,
        streamMessageId: action.payload.messageId
      });
      syncCurrentChatHistory(state);
    },
    appendAgentEvent: (state, action: PayloadAction<AppendAgentEventPayload>) => {
      appendAssistantMessage(state, {
        eventName: action.payload.eventName,
        text: action.payload.text,
        runId: state.currentRunId ?? undefined
      });
      syncCurrentChatHistory(state);
    },
    appendAgentMessagePart: (state, action: PayloadAction<AppendAgentMessagePartPayload>) => {
      const targetMessage = findAssistantMessageForStream(state, action.payload.messageId);
      if (targetMessage) {
        const existingParts = targetMessage.parts ?? [];
        const partId = action.payload.part.id;
        const existingPartIndex = partId ? existingParts.findIndex((part) => part.id === partId) : -1;
        targetMessage.parts = [...existingParts];
        if (existingPartIndex >= 0) {
          targetMessage.parts[existingPartIndex] = action.payload.part;
        } else {
          targetMessage.parts.push(action.payload.part);
        }
        targetMessage.eventName = action.payload.eventName ?? targetMessage.eventName ?? TEXT_DELTA_EVENT;
        targetMessage.streamMessageId = action.payload.messageId ?? targetMessage.streamMessageId;
        syncCurrentChatHistory(state);
        return;
      }

      appendAssistantMessage(state, {
        eventName: action.payload.eventName ?? TEXT_DELTA_EVENT,
        text: '',
        parts: [action.payload.part],
        runId: state.currentRunId ?? undefined,
        streamMessageId: action.payload.messageId
      });
      syncCurrentChatHistory(state);
    },
    clearAgentChat: (state) => {
      syncCurrentChatHistory(state);
      state.messages = [];
      state.currentRunId = null;
      state.currentConversationId = null;
      state.currentStreamId = null;
      state.createRunLoading = LoadingState.IDLE;
      state.error = null;
    },
    setSelectedAgentModel: (state, action: PayloadAction<string>) => {
      if (AGENT_MODEL_OPTIONS.some((option) => option.key === action.payload)) {
        state.selectedModelKey = action.payload;
      }
    },
    selectAgentConversation: (state, action: PayloadAction<string>) => {
      const conversation = state.chatHistoryById[action.payload];
      if (!conversation) {
        return;
      }

      syncCurrentChatHistory(state);
      state.messages = cloneChatMessages(conversation.messages);
      state.currentConversationId = conversation.id;
      state.currentStreamId = conversation.streamId ?? null;
      state.currentRunId = lastRunId(conversation.messages);
      state.createRunLoading = LoadingState.IDLE;
      state.error = null;
      moveConversationToFront(state, conversation.id);
    }
  },
  extraReducers: (builder) => {
    builder
      .addCase(createAgentRun.pending, (state, action) => {
        state.createRunLoading = LoadingState.PENDING;
        state.error = null;
        state.messages.push({
          id: nextMessageID(state, 'user-message'),
          role: 'user',
          text: action.meta.arg.message
        });
        syncCurrentChatHistory(state);
      })
      .addCase(createAgentRun.fulfilled, (state, action) => {
        state.createRunLoading = LoadingState.SUCCESS;
        state.error = null;
        state.runsById[action.payload.id] = action.payload;
        state.currentRunId = action.payload.id;
        state.currentConversationId = action.payload.conversationId ?? state.currentConversationId;
        state.currentStreamId = resolveAgentRunStreamId(action.payload) ?? state.currentStreamId;

        if (action.payload.finalMessage) {
          appendAssistantMessage(state, {
            eventName: 'agent::chat::message',
            text: action.payload.finalMessage,
            runId: action.payload.id
          });
        }

        if (action.payload.conversationId) {
          upsertChatHistory(state, action.payload.conversationId, {
            title: action.payload.conversation?.title ?? undefined,
            metadata: action.payload.conversation?.metadata ?? action.payload.metadata,
            streamId: resolveAgentRunStreamId(action.payload) ?? undefined,
            updatedAt: action.payload.updatedAt ?? action.payload.createdAt
          });
        }
      })
      .addCase(createAgentRun.rejected, (state, action) => {
        const errorMessage = action.error.message ?? 'Failed to start agent run';
        state.createRunLoading = LoadingState.ERROR;
        state.error = errorMessage;
        appendAssistantMessage(state, {
          eventName: 'agent::chat::error',
          text: errorMessage
        });
        syncCurrentChatHistory(state);
      })
      .addCase(listAgentConversations.fulfilled, (state, action) => {
        mergeConversationHistory(state, action.payload);
      })
      .addCase(fetchAgentConversationMessages.fulfilled, (state, action) => {
        const existing = state.chatHistoryById[action.payload.conversationId];
        const messages = chatMessagesFromConversationMessages(action.payload.messages);
        state.chatHistoryById[action.payload.conversationId] = {
          id: action.payload.conversationId,
          title: existing?.title ?? chatTitleFromMessages(messages),
          messages,
          metadata: existing?.metadata,
          streamId: existing?.streamId ?? action.payload.conversationId,
          updatedAt: existing?.updatedAt
        };

        if (state.currentConversationId === action.payload.conversationId) {
          state.messages = cloneChatMessages(messages);
          state.currentRunId = lastRunId(messages);
          state.currentStreamId = state.chatHistoryById[action.payload.conversationId].streamId ?? null;
        }
      })
      .addCase(deleteAgentConversation.fulfilled, (state, action) => {
        delete state.chatHistoryById[action.payload];
        state.chatHistoryOrder = state.chatHistoryOrder.filter((id) => id !== action.payload);
        if (state.currentConversationId === action.payload) {
          state.messages = [];
          state.currentRunId = null;
          state.currentConversationId = null;
          state.currentStreamId = null;
          state.createRunLoading = LoadingState.IDLE;
        }
      });
  }
});

export const {
  appendAgentEvent,
  appendAgentMessagePart,
  appendAgentTextDelta,
  clearAgentChat,
  selectAgentConversation,
  setSelectedAgentModel
} = agentSlice.actions;

export const selectAgentMessages = (state: RootState) => state.agent.messages;
export const selectAgentChatHistory = (state: RootState) =>
  Array.from(new Set(state.agent.chatHistoryOrder))
    .map((id) => state.agent.chatHistoryById[id])
    .filter((conversation): conversation is AgentChatHistoryItem => Boolean(conversation));
export const selectAgentCreateRunLoading = (state: RootState) => state.agent.createRunLoading;
export const selectAgentError = (state: RootState) => state.agent.error;
export const selectCurrentAgentConversationId = (state: RootState) => state.agent.currentConversationId;
export const selectCurrentAgentStreamId = (state: RootState) => state.agent.currentStreamId;
export const selectSelectedAgentModelKey = (state: RootState) => state.agent.selectedModelKey;

export default agentSlice.reducer;
