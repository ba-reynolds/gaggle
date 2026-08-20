import { useInfiniteQuery } from '@tanstack/react-query';
import { getNotifications } from '@/api/notifications';
import type { Envelope, Notification, PaginatedFeedResponse } from '@/types/api';

export const notificationsQueryKey = ['notifications'] as const;

export const notificationsInfiniteOptions = {
  queryKey: notificationsQueryKey,
  queryFn: ({ pageParam }: { pageParam: string | undefined }) => getNotifications(pageParam),
  initialPageParam: undefined as string | undefined,
  getNextPageParam: (last: Envelope<PaginatedFeedResponse<Notification>>) =>
    last.data.next_cursor ?? undefined,
};

export const useNotificationsQuery = () => useInfiniteQuery(notificationsInfiniteOptions);