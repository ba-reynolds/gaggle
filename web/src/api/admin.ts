import api from "@/lib/api";
import type { AdminMetrics, Badge, CreateBadgePayload, Envelope, HistoryRange, MetricsHistory, ViewRange } from "@/types/api";

export const getAdminMetrics = async (): Promise<Envelope<AdminMetrics>> => {
  const response = await api.get<Envelope<AdminMetrics>>('/admin/metrics');
  return response.data;
};

export const VIEW_DAYS: Record<ViewRange, number> = { "14d": 14, "30d": 30, "90d": 90 };

export const getAdminMetricsHistory = async (range: HistoryRange, days: ViewRange): Promise<Envelope<MetricsHistory>> => {
  const response = await api.get<Envelope<MetricsHistory>>('/admin/metrics/history', {
    params: { range, days: VIEW_DAYS[days] },
  });
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