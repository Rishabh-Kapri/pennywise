import { useEffect, useRef } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { config } from '@/config/env';
import { refreshAccessToken, selectAccessToken } from '@/features/auth/store';
import { selectSelectedBudget } from '@/features/budget';
import { fetchAllTransaction } from '@/features/transactions/store';
import { parseJWT } from '@/utils/auth.utils';

const RECONNECT_DELAY_MS = 3000;
const TOKEN_EXPIRY_BUFFER_MS = 30000;

type WebSocketMessage = {
  eventName: string;
  budgetId: string;
  data?: unknown;
};

function getWebSocketUrl(accessToken: string, budgetId: string) {
  const apiUrl = new URL(config.apiBaseUrl);
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
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);

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
      };

      socket.onmessage = (event) => {
        try {
          console.log('message received', event);
          const message = JSON.parse(event.data) as WebSocketMessage;

          if (message.budgetId !== budgetId) {
            return;
          }

          if (message.eventName === 'transaction.created' || message.eventName === 'transaction::created') {
            dispatch(fetchAllTransaction());
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
    };
  }, [accessToken, selectedBudget?.id, dispatch]);

  return null;
}
