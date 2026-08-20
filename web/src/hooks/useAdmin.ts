import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createBadge, deleteBadge, getAdminMetrics, grantBadge, listBadgeCatalog, revokeBadge, updateBadge } from '@/api/admin';
import type { Badge, CreateBadgePayload } from '@/types/api';

export const useAdminMetrics = () =>
  useQuery({
    queryKey: ['admin-metrics'],
    queryFn: getAdminMetrics,
    refetchInterval: 5000,
  });

export const useBadgeCatalog = () =>
  useQuery({
    queryKey: ['badge-catalog'],
    queryFn: listBadgeCatalog,
  });

export const useCreateBadge = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateBadgePayload) => createBadge(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['badge-catalog'] });
    },
  });
};

export const useUpdateBadge = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ badgeId, payload }: { badgeId: number; payload: CreateBadgePayload }) =>
      updateBadge(badgeId, payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['badge-catalog'] });
    },
  });
};

export const useDeleteBadge = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (badgeId: number) => deleteBadge(badgeId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['badge-catalog'] });
    },
  });
};

export const useGrantBadge = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ username, badgeId }: { username: string; badgeId: number }) =>
      grantBadge(username, badgeId),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['profile', variables.username] });
      void queryClient.invalidateQueries({ queryKey: ['pinned-post'] });
    },
  });
};

export const useRevokeBadge = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ username, badgeId }: { username: string; badgeId: number }) =>
      revokeBadge(username, badgeId),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['profile', variables.username] });
    },
  });
};

export type { Badge };