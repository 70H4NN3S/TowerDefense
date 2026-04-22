import { api } from '../client.ts';
import type { ChatMessage } from '../types.ts';

interface GetMessagesParams {
  /** Exclusive cursor — return only messages before this RFC 3339 timestamp. */
  before?: string;
  /** Number of messages to return (1–100, default 50). */
  limit?: number;
}

/** Fetch paginated message history for a channel, newest-first. */
export function getMessages(
  channelId: string,
  params: GetMessagesParams = {},
): Promise<{ messages: ChatMessage[] }> {
  const query = new URLSearchParams();
  if (params.before !== undefined) query.set('before', params.before);
  if (params.limit !== undefined) query.set('limit', String(params.limit));
  const qs = query.size > 0 ? `?${query.toString()}` : '';
  return api.get<{ messages: ChatMessage[] }>(`/v1/chat/channels/${channelId}/messages${qs}`);
}

/** Send a message to a channel (rate-limited: 10 msg / 10 s per user-channel). */
export function sendMessage(channelId: string, body: string): Promise<{ message: ChatMessage }> {
  return api.post<{ message: ChatMessage }>(`/v1/chat/channels/${channelId}/messages`, { body });
}
