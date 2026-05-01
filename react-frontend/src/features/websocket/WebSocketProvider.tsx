import { useEffect, useRef } from 'react';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { config } from '@/config/env';
import { selectAccessToken } from '@/features/auth/store';
import { selectSelectedBudget } from '@/features/budget';
import { fetchAllTransaction } from '@/features/transactions/store';

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

  useEffect(() => {
    const budgetId = selectedBudget?.id;
    if (!accessToken || !budgetId) {
      socketRef.current?.close();
      socketRef.current = null;
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
    };

    return () => {
      socket.close();
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
    };
  }, [accessToken, selectedBudget?.id, dispatch]);

  return null;
}
