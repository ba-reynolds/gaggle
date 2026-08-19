import api from '@/lib/api';
import type { Envelope, PaginatedFeedResponse, Post, Trend, UserProfileResponse } from '@/types/api';

export interface PostSearchFilters {
  from?: string;
  hashtag?: string;
  media?: boolean;
  minLikes?: number;
  includeReplies?: boolean;
  since?: string;
  until?: string;
}

export const searchPosts = async (
  query: string,
  filters: PostSearchFilters = {},
  cursor?: string
): Promise<Envelope<PaginatedFeedResponse<Post>>> => {
  const params: Record<string, string | number> = { q: query, type: 'posts', cursor: cursor ?? '', limit: 20 };
  if (filters.from) params.from = filters.from;
  if (filters.hashtag) params.hashtag = filters.hashtag;
  if (filters.media) params.has_media = 'true';
  if (filters.minLikes !== undefined && filters.minLikes > 0) params.min_likes = filters.minLikes;
  if (filters.includeReplies) params.include_replies = 'true';
  if (filters.since) params.since = filters.since;
  if (filters.until) params.until = filters.until;
  const response = await api.get<Envelope<PaginatedFeedResponse<Post>>>('/search', { params });
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

export const getSuggestedUsers = async (limit?: number): Promise<Envelope<{ items: UserProfileResponse[] }>> => {
  const response = await api.get<Envelope<{ items: UserProfileResponse[] }>>('/users/suggested', {
    params: { limit },
  });
  return response.data;
};
