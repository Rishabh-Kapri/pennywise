import { useCallback, useEffect, useRef } from 'react';
import { config } from '../../config/env';
import { refreshAccessToken, selectAccessToken } from '../auth/store/authSlice';
import { selectSelectedBudget } from '../budget/store/budgetSlice';
import { fetchAllTransactions } from '../transactions/store/transactionSlice';
import { parseJWT } from '../../utils/auth';
import { useAppDispatch, useAppSelector } from '../../app/hooks';
import {
  appendAgentEvent,
  appendAgentMessagePart,
  appendAgentTextDelta,
  selectCurrentAgentStreamId
} from '../agent/store/agentSlice';
import type { AgentEventMessageData, MessagePart } from '../agent/types';

const RECONNECT_DELAY_MS = 3000;
const TOKEN_EXPIRY_BUFFER_MS = 30000;
const AGENT_CHAT_STREAM_EVENT = 'pennywise::agent::chat::stream';
const AGENT_CHAT_SUBSCRIBE_EVENT = 'pennywise::agent::chat::subscribe';

type WebSocketMessage = {
  eventName: string;
  budgetId: string;
  userId?: string;
  streamId?: string;
  roomId?: string;
  data?: unknown;
};

type WebSocketSubscriptionMessage = {
  eventName: typeof AGENT_CHAT_SUBSCRIBE_EVENT;
  budgetId: string;
  userId: string;
  roomId: string;
  data: {
    kind: 'agent-stream';
    streamId: string;
    budgetId: string;
    userId: string;
    roomId: string;
  };
};

function getWebSocketUrl(accessToken: string, budgetId: string) {
  const apiUrl = new URL(config.apiBaseUrl);
  const protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  const params = new URLSearchParams({ token: accessToken, budgetId });
  return `${protocol}//${apiUrl.host}/ws?${params.toString()}`;
}

function getAgentRoomId(budgetId: string, userId: string, streamId: string) {
  return `${budgetId}:${userId}:chat/${streamId}`;
}

function buildAgentRoomSubscription(budgetId: string, userId: string, streamId: string): WebSocketSubscriptionMessage {
  const roomId = getAgentRoomId(budgetId, userId, streamId);

  return {
    eventName: AGENT_CHAT_SUBSCRIBE_EVENT,
    budgetId,
    userId,
    roomId,
    data: {
      kind: 'agent-stream',
      streamId,
      budgetId,
      userId,
      roomId
    }
  };
}

function parseRecordData(data: unknown): Record<string, unknown> | null {
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data) as unknown;
      return parseRecordData(parsed);
    } catch {
      return null;
    }
  }

  if (data && typeof data === 'object' && !Array.isArray(data)) {
    return data as Record<string, unknown>;
  }

  return null;
}

function valueFromData(data: unknown, key: string) {
  const record = parseRecordData(data);
  if (!record || !(key in record)) {
    return undefined;
  }

  const value = record[key];
  return typeof value === 'string' ? value : undefined;
}

function messageMatchesAgentRoom(message: WebSocketMessage, budgetId: string, userId?: string, streamId?: string | null) {
  if (message.budgetId !== budgetId || !streamId) {
    return false;
  }

  const messageUserId = message.userId ?? valueFromData(message.data, 'userId');
  if (userId && messageUserId && messageUserId !== userId) {
    return false;
  }

  const messageStreamId = message.streamId ?? valueFromData(message.data, 'streamId');
  const messageRoomId = message.roomId ?? valueFromData(message.data, 'roomId');
  const expectedRoomId = userId ? getAgentRoomId(budgetId, userId, streamId) : undefined;

  if (messageStreamId && messageStreamId !== streamId) {
    return false;
  }

  if (expectedRoomId && messageRoomId && messageRoomId !== expectedRoomId) {
    return false;
  }

  return true;
}

function parseAgentStreamData(data: unknown) {
  const record = parseRecordData(data);
  return record ? (record as AgentEventMessageData) : null;
}

function agentStreamMessageId(data: AgentEventMessageData | null) {
  if (!data) {
    return undefined;
  }

  const fallbackMessageId = (data as { messageId?: unknown }).messageId;
  if (typeof data.id === 'string' && data.id) {
    return data.id;
  }
  if (typeof fallbackMessageId === 'string' && fallbackMessageId) {
    return fallbackMessageId;
  }
  return undefined;
}

function streamToolMessage(value: unknown) {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {};
}

function toolCallStartPart(message: unknown): MessagePart {
  const toolMessage = streamToolMessage(message);
  return {
    type: 'TOOL_CALL',
    id: typeof toolMessage.id === 'string' ? toolMessage.id : undefined,
    displayName: typeof toolMessage.displayName === 'string' ? toolMessage.displayName : 'Using tool'
  };
}

function formatAgentEventData(data: unknown) {
  if (typeof data === 'string') {
    const record = parseRecordData(data);
    if (!record) {
      return data;
    }
    return formatAgentEventData(record);
  }

  const record = parseRecordData(data);
  if (!record) {
    return '';
  }

  const value = record.message ?? record.text ?? record.delta ?? record.content;
  if (typeof value === 'string') {
    return value;
  }

  return JSON.stringify(record);
}

function isTokenExpiringSoon(accessToken: string) {
  const parsed = parseJWT(accessToken);
  if (!parsed?.exp) return true;
  return Date.now() >= parsed.exp * 1000 - TOKEN_EXPIRY_BUFFER_MS;
}

export function WebSocketProvider() {
  const dispatch = useAppDispatch();
  const accessToken = useAppSelector(selectAccessToken);
  const currentAgentStreamId = useAppSelector(selectCurrentAgentStreamId);
  const currentUser = useAppSelector((state) => state.auth.user);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const budgetId = selectedBudget?.id;
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastSubscribedAgentRoomRef = useRef<string | null>(null);
  const currentAgentStreamIdRef = useRef<string | null>(null);
  const currentBudgetIdRef = useRef<string | undefined>(undefined);
  const currentUserIdRef = useRef<string | undefined>(undefined);

  currentAgentStreamIdRef.current = currentAgentStreamId;
  currentBudgetIdRef.current = budgetId;
  currentUserIdRef.current = currentUser?.id;

  const subscribeToAgentRoom = useCallback((streamId?: string | null, activeBudgetId?: string, userId?: string) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN || !activeBudgetId || !userId || !streamId) {
      return;
    }

    const roomId = getAgentRoomId(activeBudgetId, userId, streamId);
    if (lastSubscribedAgentRoomRef.current === roomId) {
      return;
    }

    socket.send(JSON.stringify(buildAgentRoomSubscription(activeBudgetId, userId, streamId)));
    lastSubscribedAgentRoomRef.current = roomId;
  }, []);

  useEffect(() => {
    subscribeToAgentRoom(currentAgentStreamId, budgetId, currentUser?.id);
  }, [currentAgentStreamId, budgetId, currentUser?.id, subscribeToAgentRoom]);

  useEffect(() => {
    if (!accessToken || !budgetId) {
      socketRef.current?.close();
      socketRef.current = null;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      lastSubscribedAgentRoomRef.current = null;
      return;
    }

    let shouldReconnect = true;

    const connect = async () => {
      let token = accessToken;
      if (isTokenExpiringSoon(token)) {
        try {
          token = (await dispatch(refreshAccessToken()).unwrap()).accessToken;
        } catch {
          shouldReconnect = false;
          return;
        }
      }

      const socket = new WebSocket(getWebSocketUrl(token, budgetId));
      socketRef.current = socket;
      socket.onopen = () => {
        lastSubscribedAgentRoomRef.current = null;
        subscribeToAgentRoom(currentAgentStreamIdRef.current, currentBudgetIdRef.current, currentUserIdRef.current);
      };
      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as WebSocketMessage;
          if (message.budgetId !== budgetId) {
            return;
          }

          if (
            message.eventName === 'transaction.created' ||
            message.eventName === 'transaction::created' ||
            message.eventName === 'pennywise::transaction::created'
          ) {
            dispatch(fetchAllTransactions());
          }

          if (
            message.eventName.startsWith('pennywise::agent') &&
            messageMatchesAgentRoom(message, budgetId, currentUserIdRef.current, currentAgentStreamIdRef.current)
          ) {
            const parsedData = parseAgentStreamData(message.data);
            const streamMessageId = agentStreamMessageId(parsedData);

            if (message.eventName === AGENT_CHAT_STREAM_EVENT && parsedData?.type === 'text_delta') {
              const text = typeof parsedData.message === 'string' ? parsedData.message : '';
              dispatch(appendAgentTextDelta({ text, messageId: streamMessageId }));
              return;
            }

            if (message.eventName === AGENT_CHAT_STREAM_EVENT && parsedData?.type === 'tool_call_start') {
              dispatch(
                appendAgentMessagePart({
                  messageId: streamMessageId,
                  part: toolCallStartPart(parsedData.message)
                })
              );
              return;
            }

            if (
              message.eventName === AGENT_CHAT_STREAM_EVENT &&
              parsedData?.type === 'tool_call' &&
              parsedData.message &&
              typeof parsedData.message === 'object'
            ) {
              const toolMessage = streamToolMessage(parsedData.message);
              dispatch(
                appendAgentMessagePart({
                  messageId: streamMessageId,
                  part: {
                    type: 'TOOL_CALL',
                    id: typeof toolMessage.id === 'string' ? toolMessage.id : undefined,
                    displayName: typeof toolMessage.displayName === 'string' ? toolMessage.displayName : undefined,
                    summary: typeof toolMessage.summary === 'string' ? toolMessage.summary : undefined,
                    result: toolMessage.result
                  }
                })
              );
              return;
            }

            dispatch(appendAgentEvent({ eventName: message.eventName, text: formatAgentEventData(message.data) }));
          }
        } catch {
          // Ignore malformed messages.
        }
      };
      socket.onclose = () => {
        if (!shouldReconnect) return;
        reconnectRef.current = setTimeout(() => void connect(), RECONNECT_DELAY_MS);
      };
    };

    void connect();

    return () => {
      shouldReconnect = false;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
      socketRef.current?.close();
      socketRef.current = null;
      lastSubscribedAgentRoomRef.current = null;
    };
  }, [accessToken, budgetId, dispatch, subscribeToAgentRoom]);

  return null;
}
