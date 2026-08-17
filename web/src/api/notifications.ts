import api from '@/lib/api';
import type { Envelope, Notification, PaginatedFeedResponse } from '@/types/api';

export const getNotifications = async (cursor?: string): Promise<Envelope<PaginatedFeedResponse<Notification>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<Notification>>>('/notifications', {
    params: { cursor, limit: 20 },
  });
  return response.data;
};

export const getUnreadNotificationCount = async (): Promise<Envelope<{ count: number }>> => {
  const response = await api.get<Envelope<{ count: number }>>('/notifications/unread-count');
  return response.data;
};

export const markNotificationRead = async (id: number): Promise<void> => {
  await api.post(`/notifications/${id}/read`);
};

export const markAllNotificationsRead = async (): Promise<void> => {
  await api.post('/notifications/read');
};
