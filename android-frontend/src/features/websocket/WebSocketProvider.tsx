import { useEffect, useRef } from 'react';
import { config } from '../../config/env';
import { refreshAccessToken, selectAccessToken } from '../auth/store/authSlice';
import { selectSelectedBudget } from '../budget/store/budgetSlice';
import { fetchAllTransactions } from '../transactions/store/transactionSlice';
import { parseJWT } from '../../utils/auth';
import { useAppDispatch, useAppSelector } from '../../app/hooks';

const RECONNECT_DELAY_MS = 3000;
const TOKEN_EXPIRY_BUFFER_MS = 30000;

type WebSocketMessage = {
  eventName: string;
  budgetId: string;
};

function getWebSocketUrl(accessToken: string, budgetId: string) {
  const apiUrl = new URL(config.apiBaseUrl);
  const protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  const params = new URLSearchParams({ token: accessToken, budgetId });
  return `${protocol}//${apiUrl.host}/ws?${params.toString()}`;
}

function isTokenExpiringSoon(accessToken: string) {
  const parsed = parseJWT(accessToken);
  if (!parsed?.exp) return true;
  return Date.now() >= parsed.exp * 1000 - TOKEN_EXPIRY_BUFFER_MS;
}

export function WebSocketProvider() {
  const dispatch = useAppDispatch();
  const accessToken = useAppSelector(selectAccessToken);
  const budgetId = useAppSelector(selectSelectedBudget)?.id;
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!accessToken || !budgetId) {
      socketRef.current?.close();
      socketRef.current = null;
      if (reconnectRef.current) clearTimeout(reconnectRef.current);
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
      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as WebSocketMessage;
          if (message.budgetId === budgetId && (message.eventName === 'transaction.created' || message.eventName === 'transaction::created')) {
            dispatch(fetchAllTransactions());
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
    };
  }, [accessToken, budgetId, dispatch]);

  return null;
}
