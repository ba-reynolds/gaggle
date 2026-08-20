import api from "@/lib/api";
import type { AdminMetrics, Badge, CreateBadgePayload, Envelope } from "@/types/api";

export const getAdminMetrics = async (): Promise<Envelope<AdminMetrics>> => {
  const response = await api.get<Envelope<AdminMetrics>>('/admin/metrics');
  return response.data;
};

export const listBadgeCatalog = async (): Promise<Envelope<Badge[]>> => {
  const response = await api.get<Envelope<Badge[]>>('/admin/badges');
  return response.data;
};

export const createBadge = async (payload: CreateBadgePayload): Promise<Envelope<Badge>> => {
  const response = await api.post<Envelope<Badge>>('/admin/badges', payload);
  return response.data;
};

export const updateBadge = async (badgeId: number, payload: CreateBadgePayload): Promise<Envelope<Badge>> => {
  const response = await api.patch<Envelope<Badge>>(`/admin/badges/${badgeId}`, payload);
  return response.data;
};

export const deleteBadge = async (badgeId: number): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.delete<Envelope<{ success: boolean }>>(`/admin/badges/${badgeId}`);
  return response.data;
};

export const grantBadge = async (username: string, badgeId: number): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.post<Envelope<{ success: boolean }>>(`/admin/users/${username}/badges/${badgeId}`);
  return response.data;
};

export const revokeBadge = async (username: string, badgeId: number): Promise<Envelope<{ success: boolean }>> => {
  const response = await api.delete<Envelope<{ success: boolean }>>(`/admin/users/${username}/badges/${badgeId}`);
  return response.data;
};