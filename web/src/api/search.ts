import api from '@/lib/api';
import type { Envelope, PaginatedFeedResponse, Post, Trend, UserProfileResponse } from '@/types/api';

export const searchPosts = async (query: string, cursor?: string): Promise<Envelope<PaginatedFeedResponse<Post>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<Post>>>('/search', {
    params: { q: query, type: 'posts', cursor, limit: 20 },
  });
  return response.data;
};

export const searchUsers = async (query: string): Promise<Envelope<{ items: UserProfileResponse[] }>> => {
  const response = await api.get<Envelope<{ items: UserProfileResponse[] }>>('/search', {
    params: { q: query, type: 'users', limit: 20 },
  });
  return response.data;
};

export const getHashtagPosts = async (tag: string, cursor?: string): Promise<Envelope<PaginatedFeedResponse<Post>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<Post>>>(`/hashtags/${encodeURIComponent(tag)}/posts`, {
    params: { cursor, limit: 20 },
  });
  return response.data;
};

export const getTrends = async (): Promise<Envelope<Trend[]>> => {
  const response = await api.get<Envelope<Trend[]>>('/trends');
  return response.data;
};
