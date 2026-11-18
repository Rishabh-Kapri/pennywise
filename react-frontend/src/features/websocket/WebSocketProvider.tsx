import { useCallback, useEffect, useRef } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { config } from '@/config/env';
import { selectCurrentAgentStreamId } from '@/features/agent/store/agentSlice';
import { selectAccessToken, selectUser } from '@/features/auth/store/authSlice';
import { selectSelectedBudget } from '@/features/budget/store/budgetSlice';
import { jobsMayHaveChanged, jobUpdated } from '@/features/jobs/store/jobSlice';
import type { JobUpdate } from '@/features/jobs/types/job.types';
import { fetchAllTransaction } from '@/features/transactions/store/transactionSlice';
import { apiClient } from '@/utils/api';
import { parseJWT } from '@/utils/auth.utils';
import { agentRoomId, agentRoomSubscription, isForAgentRoom, publishAgentMessage } from '@/features/agent/protocol';
import { JOB_UPDATE_EVENT, TRANSACTION_CREATED_EVENT, type WebSocketMessage } from './events';

const RECONNECT_DELAY_MS = 3000;
const TOKEN_EXPIRY_BUFFER_MS = 30000;

function getWebSocketUrl(accessToken: string, budgetId: string) {
  const apiUrl = new URL(config.apiBaseUrl, window.location.origin);
  const protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  const params = new URLSearchParams({
    token: accessToken,
    budgetId,
  });

  return `${protocol}//${apiUrl.host}/ws?${params.toString()}`;
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
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN || !budgetId || !userId || !streamId) {
      return;
    }

    const roomId = agentRoomId(budgetId, userId, streamId);
    if (lastSubscribedAgentRoomRef.current === roomId) {
      return;
    }

    socket.send(JSON.stringify(agentRoomSubscription(budgetId, userId, streamId)));
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

    const scheduleReconnect = () => {
      reconnectTimerRef.current = window.setTimeout(() => {
        reconnectTimerRef.current = null;
        void connect();
      }, RECONNECT_DELAY_MS);
    };

    const connect = async () => {
      if (!shouldReconnect) {
        return;
      }

      let token = accessToken;
      if (isTokenExpiringSoon(token)) {
        try {
          token = await apiClient.refreshSession();
        } catch (error) {
          // A dead session clears the token, which ends this effect. Anything
          // else is a server that's away, so keep trying.
          console.error('failed to refresh token for websocket', error);
          if (shouldReconnect) {
            scheduleReconnect();
          }
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
        // Updates sent while the socket was down are gone.
        dispatch(jobsMayHaveChanged());
        subscribeToAgentRoom(currentAgentStreamIdRef.current, currentBudgetIdRef.current, currentUserIdRef.current);
      };

      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as WebSocketMessage;

          if (message.budgetId !== budgetId) {
            return;
          }

          if (message.eventName === TRANSACTION_CREATED_EVENT) {
            dispatch(fetchAllTransaction());
          }

          if (message.eventName === JOB_UPDATE_EVENT && message.data) {
            const update = (typeof message.data === 'string' ? JSON.parse(message.data) : message.data) as JobUpdate;
            if (update?.jobId) {
              dispatch(jobUpdated(update));
            }
          }

          if (
            message.eventName.startsWith('pennywise::agent') &&
            isForAgentRoom(message, budgetId, currentUserIdRef.current, currentAgentStreamIdRef.current)
          ) {
            publishAgentMessage(message);
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

        scheduleReconnect();
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
