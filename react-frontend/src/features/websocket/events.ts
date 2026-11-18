// Mirror the Event* constants in backend/shared/model/websocket.go.
export const TRANSACTION_CREATED_EVENT = 'pennywise::transaction::created';
export const JOB_UPDATE_EVENT = 'pennywise::jobs::update';

export type WebSocketMessage = {
  eventName: string;
  budgetId: string;
  userId?: string;
  streamId?: string;
  roomId?: string;
  conversationId?: string;
  data?: string;
};
