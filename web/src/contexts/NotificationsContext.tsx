import { createContext, useContext, useEffect, useMemo, type ReactNode } from 'react';
import { useQuery, useQueryClient, type InfiniteData } from '@tanstack/react-query';
import { getUnreadNotificationCount, markAllNotificationsRead, markNotificationRead } from '@/api/notifications';
import { useAuth } from '@/contexts/AuthContext';
import type { Envelope, Notification, PaginatedFeedResponse } from '@/types/api';

type NotificationEnvelope = Envelope<PaginatedFeedResponse<Notification>>;

interface NotificationsContextValue {
  unreadCount: number;
  markRead: (id: number) => Promise<void>;
  markAllRead: () => Promise<void>;
}

const NotificationsContext = createContext<NotificationsContextValue | undefined>(undefined);

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const { token } = useAuth();
  const queryClient = useQueryClient();
  const unreadQuery = useQuery({
    queryKey: ['notifications-unread-count'],
    queryFn: async () => (await getUnreadNotificationCount()).data,
    enabled: typeof token === 'string',
  });

  useEffect(() => {
    if (typeof token !== 'string') {
      return;
    }

    const source = new EventSource('/api/v1/stream', { withCredentials: true });
    const refreshNotifications = () => {
      void queryClient.invalidateQueries({ queryKey: ['notifications-unread-count'] });
      void queryClient.invalidateQueries({ queryKey: ['notifications'] });
    };
    const refreshFeed = () => {
      void queryClient.invalidateQueries({ queryKey: ['feed'] });
    };
    const refreshDms = () => {
      void queryClient.invalidateQueries({ queryKey: ['dm-unread-count'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-conversations'] });
      // Also refetch any open conversation so incoming messages appear live
      // instead of only after a manual reload.
      void queryClient.invalidateQueries({ queryKey: ['dm-messages'] });
    };
    source.addEventListener('notification.new', refreshNotifications);
    source.addEventListener('feed.post_created', refreshFeed);
    source.addEventListener('dm.new', refreshDms);
    source.addEventListener('dm.unread', refreshDms);
    source.addEventListener('stream.resync', () => {
      refreshNotifications();
      refreshFeed();
      refreshDms();
    });

    return () => {
      source.close();
    };
  }, [queryClient, token]);

  const value = useMemo<NotificationsContextValue>(() => ({
    unreadCount: unreadQuery.data?.count ?? 0,
    markRead: async (id) => {
      await markNotificationRead(id);
      await queryClient.invalidateQueries({ queryKey: ['notifications-unread-count'] });
      await queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    markAllRead: async () => {
      // Optimistically mark everything read so the page responds instantly;
      // the refetch below reconciles with the server.
      queryClient.setQueryData(['notifications-unread-count'], { count: 0 });
      queryClient.setQueriesData<InfiniteData<NotificationEnvelope>>(
        { queryKey: ['notifications'] },
        (old) => {
          if (!old) return old;
          const now = new Date().toISOString();
          return {
            ...old,
            pages: old.pages.map(page => ({
              ...page,
              data: {
                ...page.data,
                items: page.data.items.map(n => ({
                  ...n,
                  read_at: n.read_at ?? now,
                })),
              },
            })),
          };
        }
      );
      await markAllNotificationsRead();
      await queryClient.invalidateQueries({ queryKey: ['notifications-unread-count'] });
      await queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  }), [queryClient, unreadQuery.data?.count]);

  return <NotificationsContext.Provider value={value}>{children}</NotificationsContext.Provider>;
}

export function useNotifications() {
  const context = useContext(NotificationsContext);
  if (!context) throw new Error('useNotifications must be used within NotificationsProvider');
  return context;
}
