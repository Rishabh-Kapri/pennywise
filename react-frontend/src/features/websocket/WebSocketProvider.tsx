import { useCallback, useEffect, useRef } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { config } from '@/config/env';
import { selectCurrentAgentStreamId } from '@/features/agent/store';
import { refreshAccessToken, selectAccessToken, selectUser } from '@/features/auth/store';
import { selectSelectedBudget } from '@/features/budget';
import { fetchAllTransaction } from '@/features/transactions/store';
import { parseJWT } from '@/utils/auth.utils';
import {
  AGENT_CHAT_WEBSOCKET_EVENT,
  AGENT_CHAT_SUBSCRIBE_EVENT,
  type WebSocketMessage,
  type WebSocketSubscriptionMessage,
} from './events';

const RECONNECT_DELAY_MS = 3000;
const TOKEN_EXPIRY_BUFFER_MS = 30000;

function getWebSocketUrl(accessToken: string, budgetId: string) {
  const apiUrl = new URL(config.apiBaseUrl);
  const protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  const params = new URLSearchParams({
    token: accessToken,
    budgetId,
  });

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
      roomId,
    },
  };
}

function valueFromData(data: unknown, key: string) {
  if (!data || typeof data !== 'object' || !(key in data)) {
    return undefined;
  }

  const value = (data as Record<string, unknown>)[key];
  return typeof value === 'string' ? value : undefined;
}

function messageMatchesAgentRoom(
  message: WebSocketMessage,
  budgetId: string,
  userId?: string,
  streamId?: string | null,
) {
  if (message.budgetId !== budgetId) {
    return false;
  }

  const messageUserId = message.userId ?? valueFromData(message.data, 'userId');
  if (userId && messageUserId && messageUserId !== userId) {
    return false;
  }

  const messageStreamId = message.streamId ?? valueFromData(message.data, 'streamId');
  const messageRoomId = message.roomId ?? valueFromData(message.data, 'roomId');
  if (!streamId) {
    return false;
  }

  const expectedRoomId = userId ? getAgentRoomId(budgetId, userId, streamId) : undefined;

  if (messageStreamId && messageStreamId !== streamId) {
    return false;
  }

  if (expectedRoomId && messageRoomId && messageRoomId !== expectedRoomId) {
    return false;
  }

  return true;
}

function isTokenExpiringSoon(accessToken: string) {
  const parsedToken = parseJWT(accessToken);
  if (!parsedToken?.exp) {
    return true;
  }

  return Date.now() >= parsedToken.exp * 1000 - TOKEN_EXPIRY_BUFFER_MS;
}

export function WebSocketProvider() {
  const dispatch = useAppDispatch();
  const accessToken = useAppSelector(selectAccessToken);
  const currentAgentStreamId = useAppSelector(selectCurrentAgentStreamId);
  const currentUser = useAppSelector(selectUser);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);
  const lastSubscribedAgentRoomRef = useRef<string | null>(null);
  const currentAgentStreamIdRef = useRef<string | null>(null);
  const currentBudgetIdRef = useRef<string | undefined>(undefined);
  const currentUserIdRef = useRef<string | undefined>(undefined);

  currentAgentStreamIdRef.current = currentAgentStreamId;
  currentBudgetIdRef.current = selectedBudget?.id;
  currentUserIdRef.current = currentUser?.id;

  const subscribeToAgentRoom = useCallback((streamId?: string | null, budgetId?: string, userId?: string) => {
    console.log('subscribing to agent room', streamId, budgetId, userId);
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN || !budgetId || !userId || !streamId) {
      return;
    }

    const roomId = getAgentRoomId(budgetId, userId, streamId);
    if (lastSubscribedAgentRoomRef.current === roomId) {
      return;
    }

    socket.send(JSON.stringify(buildAgentRoomSubscription(budgetId, userId, streamId)));
    lastSubscribedAgentRoomRef.current = roomId;
  }, []);

  useEffect(() => {
    subscribeToAgentRoom(currentAgentStreamId, selectedBudget?.id, currentUser?.id);
  }, [currentAgentStreamId, selectedBudget?.id, currentUser?.id, subscribeToAgentRoom]);

  useEffect(() => {
    const budgetId = selectedBudget?.id;
    if (!accessToken || !budgetId) {
      if (reconnectTimerRef.current) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      socketRef.current?.close();
      socketRef.current = null;
      return;
    }

    let shouldReconnect = true;

    const connect = async () => {
      if (!shouldReconnect) {
        return;
      }

      let token = accessToken;
      if (isTokenExpiringSoon(token)) {
        try {
          const refreshedToken = await dispatch(refreshAccessToken()).unwrap();
          token = refreshedToken.accessToken;
        } catch (error) {
          shouldReconnect = false;
          console.error('failed to refresh token for websocket', error);
          return;
        }
      }

      if (!shouldReconnect) {
        return;
      }

      const socket = new WebSocket(getWebSocketUrl(token, budgetId));
      socketRef.current = socket;

      socket.onopen = () => {
        console.info('websocket connected', { budgetId });
        lastSubscribedAgentRoomRef.current = null;
        subscribeToAgentRoom(currentAgentStreamIdRef.current, currentBudgetIdRef.current, currentUserIdRef.current);
      };

      socket.onmessage = (event) => {
        try {
          console.log('message received', event);
          // {
          //   "eventName": "pennywise::agent::chat::stream",
          //   "data": "{\"message\":\".\",\"type\":\"text_delta\"}",
          //   "budgetId": "2166418d-3fa2-4acc-b92c-ab9f36c18d76",
          //   "roomId": "2166418d-3fa2-4acc-b92c-ab9f36c18d76:fb7c7893-84f7-4344-a861-064985d442f7:chat/511bda0e-6e6d-40c1-80ea-053de764f601"
          // }
          const message = JSON.parse(event.data) as WebSocketMessage;

          if (message.budgetId !== budgetId) {
            return;
          }

          if (message.eventName === 'pennywise::transaction::created') {
            dispatch(fetchAllTransaction());
          }

          if (
            message.eventName.startsWith('pennywise::agent') &&
            messageMatchesAgentRoom(message, budgetId, currentUserIdRef.current, currentAgentStreamIdRef.current)
          ) {
            window.dispatchEvent(
              new CustomEvent<WebSocketMessage>(AGENT_CHAT_WEBSOCKET_EVENT, {
                detail: message,
              }),
            );
          }
        } catch (error) {
          console.error('failed to parse websocket message', error);
        }
      };

      socket.onerror = (event) => {
        console.error('websocket error', event);
      };

      socket.onclose = (event) => {
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        console.info('websocket disconnected', {
          budgetId,
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
        });

        if (!shouldReconnect) {
          return;
        }

        reconnectTimerRef.current = window.setTimeout(() => {
          reconnectTimerRef.current = null;
          void connect();
        }, RECONNECT_DELAY_MS);
      };
    };

    void connect();

    return () => {
      shouldReconnect = false;
      if (reconnectTimerRef.current) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      socketRef.current?.close();
      socketRef.current = null;
      lastSubscribedAgentRoomRef.current = null;
    };
  }, [accessToken, selectedBudget?.id, dispatch, subscribeToAgentRoom]);

  return null;
}
