export const AGENT_CHAT_WEBSOCKET_EVENT = 'pennywise:agent::chat::message';
export const AGENT_CHAT_SUBSCRIBE_EVENT = 'pennywise::agent::chat::subscribe'

export type WebSocketMessage = {
  eventName: string;
  budgetId: string;
  userId?: string;
  streamId?: string;
  roomId?: string;
  conversationId?: string;
  data?: string;
};

export type WebSocketSubscriptionMessage = {
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
