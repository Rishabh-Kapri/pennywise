import type { WebSocketMessage } from '@/features/websocket/events';

/**
 * The agent chat's side of the websocket: event names (mirroring
 * `backend/shared/model/agent.go`), its rooms, and delivery of stream
 * messages from `WebSocketProvider` to `AgentChat`.
 */

export const AGENT_SUBSCRIBE_EVENT = 'pennywise::agent::chat::subscribe';
export const AGENT_STREAM_EVENT = 'pennywise::agent::chat::stream';

/** What kind of entry a chat message is, as stored in `agentSlice`. */
export const AgentMessageKind = {
  text: 'agent::chat::text_delta',
  message: 'agent::chat::message',
  loading: 'agent::chat::loading',
  error: 'agent::chat::error',
} as const;

export interface AgentRoomSubscription {
  eventName: typeof AGENT_SUBSCRIBE_EVENT;
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
}

/** The conversation a room id (`<budget>:<user>:chat/<conversation>`) belongs to. */
export function conversationIdFromRoom(roomId: string | undefined): string | undefined {
  return roomId?.includes(':chat/') ? roomId.split(':chat/')[1] : undefined;
}

type AgentMessageListener = (message: WebSocketMessage) => void;
const listeners = new Set<AgentMessageListener>();

/** Called by the websocket for every agent message in the current room. */
export function publishAgentMessage(message: WebSocketMessage) {
  for (const listener of listeners) listener(message);
}

/** Listens for agent messages; returns the unsubscribe. */
export function subscribeAgentMessages(listener: AgentMessageListener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function agentRoomId(budgetId: string, userId: string, streamId: string) {
  return `${budgetId}:${userId}:chat/${streamId}`;
}

export function agentRoomSubscription(budgetId: string, userId: string, streamId: string): AgentRoomSubscription {
  const roomId = agentRoomId(budgetId, userId, streamId);

  return {
    eventName: AGENT_SUBSCRIBE_EVENT,
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

export function isForAgentRoom(
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

  // If we don't have a known streamId yet (e.g. the run was just dispatched
  // and the fulfilled action hasn't propagated to the ref), allow the message
  // through on a budget+user match alone so we don't drop early stream events.
  if (!streamId) {
    return true;
  }

  const messageStreamId = message.streamId ?? valueFromData(message.data, 'streamId');
  const messageRoomId = message.roomId ?? valueFromData(message.data, 'roomId');

  const expectedRoomId = userId ? agentRoomId(budgetId, userId, streamId) : undefined;

  // Only reject on a stream/room mismatch when the incoming message explicitly
  // carries those fields — if they are absent, fall through and allow.
  if (messageStreamId && messageStreamId !== streamId) {
    // Also accept if the message room contains our streamId (handles cases
    // where the message carries the new run's streamId before our ref updates).
    const roomContainsStream = messageRoomId?.includes(streamId) ?? false;
    if (!roomContainsStream) {
      return false;
    }
  }

  if (expectedRoomId && messageRoomId && messageRoomId !== expectedRoomId) {
    return false;
  }

  return true;
}
