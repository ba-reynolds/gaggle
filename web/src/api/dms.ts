import api from '@/lib/api';
import type { Conversation, Envelope, Message, PaginatedFeedResponse, SendMessagePayload } from '@/types/api';

export const getConversations = async (): Promise<Envelope<{ items: Conversation[] }>> => {
  const response = await api.get<Envelope<{ items: Conversation[] }>>('/dms/conversations');
  return response.data;
};

export const getConversation = async (conversationId: number): Promise<Envelope<Conversation>> => {
  const response = await api.get<Envelope<Conversation>>(`/dms/conversations/${conversationId}`);
  return response.data;
};

export const getConversationMessages = async (conversationId: number, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse<Message>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<Message>>>(`/dms/conversations/${conversationId}/messages`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const sendMessage = async (username: string, payload: SendMessagePayload): Promise<Envelope<Message>> => {
  const response = await api.post<Envelope<Message>>(`/dms/${username}`, payload);
  return response.data;
};

export const getDmUnreadCount = async (): Promise<Envelope<{ unread_count: number }>> => {
  const response = await api.get<Envelope<{ unread_count: number }>>('/dms/unread-count');
  return response.data;
};

export const markConversationRead = async (conversationId: number): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.post<Envelope<{ success: boolean }>>(`/dms/conversations/${conversationId}/read`);
  return response.data;
};