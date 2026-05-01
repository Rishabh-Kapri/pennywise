import { useEffect, useRef } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { config } from '@/config/env';
import { selectAccessToken } from '@/features/auth/store';
import { selectSelectedBudget } from '@/features/budget';
import { fetchAllTransaction } from '@/features/transactions/store';

const RECONNECT_DELAY_MS = 3000;

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

    const connect = () => {
      if (!shouldReconnect) {
        return;
      }

      const socket = new WebSocket(getWebSocketUrl(accessToken, budgetId));
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

          if (message.eventName === 'transaction::created') {
            dispatch(fetchAllTransaction(''));
          }
        } catch (error) {
          console.error('failed to parse websocket message', error);
        }
      };

      socket.onerror = (event) => {
        console.error('websocket error', event);
      };

      socket.onclose = () => {
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        console.info('websocket disconnected', { budgetId });

        if (!shouldReconnect) {
          return;
        }

        reconnectTimerRef.current = window.setTimeout(() => {
          reconnectTimerRef.current = null;
          connect();
        }, RECONNECT_DELAY_MS);
      };
    };

    connect();

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
