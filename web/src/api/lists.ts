import api from '@/lib/api';
import type { CreateListPayload, Envelope, List, PaginatedFeedResponse, Post, UserProfileResponse } from '@/types/api';

export const getMyLists = async (): Promise<Envelope<List[]>> => {
  const response = await api.get<Envelope<List[]>>('/lists');
  return response.data;
};

export const getUserLists = async (username: string): Promise<Envelope<List[]>> => {
  const response = await api.get<Envelope<List[]>>(`/users/${username}/lists`);
  return response.data;
};

export const getList = async (listId: number): Promise<Envelope<List>> => {
  const response = await api.get<Envelope<List>>(`/lists/${listId}`);
  return response.data;
};

export const createList = async (payload: CreateListPayload): Promise<Envelope<List>> => {
  const response = await api.post<Envelope<List>>('/lists', payload);
  return response.data;
};

export const updateList = async (listId: number, payload: CreateListPayload): Promise<Envelope<List>> => {
  const response = await api.patch<Envelope<List>>(`/lists/${listId}`, payload);
  return response.data;
};

export const deleteList = async (listId: number): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.delete<Envelope<{ success: boolean }>>(`/lists/${listId}`);
  return response.data;
};

export const getListFeed = async (listId: number, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse<Post>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<Post>>>(`/lists/${listId}/feed`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getListMembers = async (listId: number, cursor?: string, limit?: number): Promise<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>> => {
  const response = await api.get<Envelope<{ items: UserProfileResponse[]; next_cursor: string | null; has_more: boolean }>>(`/lists/${listId}/members`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const addUserToList = async (listId: number, username: string): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.post<Envelope<{ success: boolean }>>(`/lists/${listId}/members/${username}`);
  return response.data;
};

export const removeUserFromList = async (listId: number, username: string): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.delete<Envelope<{ success: boolean }>>(`/lists/${listId}/members/${username}`);
  return response.data;
};