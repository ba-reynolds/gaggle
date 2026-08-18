import { getNotifications } from '@/api/notifications';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useNotifications } from '@/contexts/NotificationsContext';
import { useInfiniteQuery } from '@tanstack/react-query';
import { Bell, CheckCheck, Heart, MessageCircle, Repeat2, UserPlus } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { getMediaUrl } from '@/util/media';
import { formatPostDate } from '@/util/date';

function notificationText(type: string) {
  switch (type) {
    case 'like': return 'liked your post';
    case 'repost': return 'reposted your post';
    case 'quote': return 'quoted your post';
    case 'reply': return 'replied to your post';
    case 'follow': return 'started following you';
    case 'mention': return 'mentioned you';
    default: return 'interacted with you';
  }
}

function notificationIcon(type: string) {
  if (type === 'like') return <Heart className="h-4 w-4 text-rose-500 fill-rose-500" />;
  if (type === 'repost') return <Repeat2 className="h-4 w-4 text-emerald-500" />;
  if (type === 'reply' || type === 'quote') return <MessageCircle className="h-4 w-4 text-sky-500" />;
  if (type === 'follow') return <UserPlus className="h-4 w-4 text-violet-500" />;
  return <Bell className="h-4 w-4 text-primary" />;
}

export default function NotificationsPage() {
  const navigate = useNavigate();
  const { markRead, markAllRead } = useNotifications();
  const query = useInfiniteQuery({
    queryKey: ['notifications'],
    queryFn: ({ pageParam }) => getNotifications(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.data.next_cursor ?? undefined,
  });
  const notifications = query.data?.pages.flatMap((page) => page.data.items) ?? [];

  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-10 flex items-center justify-between border-b border-border p-4 backdrop-blur">
        <div>
          <h1 className="text-xl font-bold text-primary">Notifications</h1>
          <p className="text-sm text-muted-foreground">Stay close to what is happening.</p>
        </div>
        <Button variant="ghost" size="sm" onClick={() => void markAllRead()}>
          <CheckCheck className="mr-2 h-4 w-4" /> Mark all read
        </Button>
      </header>
      {query.isLoading ? (
        <div className="space-y-3 p-4">{[1, 2, 3].map((item) => <Skeleton key={item} className="h-20 w-full" />)}</div>
      ) : notifications.length === 0 ? (
        <div className="flex flex-col items-center gap-2 p-12 text-center text-muted-foreground">
          <Bell className="h-10 w-10" />
          <p className="font-medium">You are all caught up.</p>
        </div>
      ) : (
        <div className="divide-y divide-border">
          {notifications.map((notification) => (
            <button
              key={notification.id}
              className={`flex w-full items-start gap-3 p-4 text-left transition hover:bg-muted/60 ${notification.read_at ? '' : 'bg-primary/5'}`}
              onClick={() => {
                void markRead(notification.id);
                if (notification.post_id) navigate(`/post/${notification.post_id}`);
                else navigate(`/profile/${notification.actor.username}`);
              }}
            >
              <div className="mt-1">{notificationIcon(notification.type)}</div>
              <Avatar className="h-10 w-10">
                <AvatarImage src={getMediaUrl(notification.actor.profile_picture_uuid)} />
                <AvatarFallback>{notification.actor.display_name?.[0] ?? notification.actor.username[0]}</AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <p className="text-sm text-primary">
                  <strong>{notification.actor.display_name || notification.actor.username}</strong>{' '}
                  {notificationText(notification.type)}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">{formatPostDate(notification.created_at)}</p>
              </div>
              {!notification.read_at && <span className="mt-2 h-2 w-2 rounded-full bg-primary" />}
            </button>
          ))}
          {query.hasNextPage && <Button variant="ghost" className="m-4 w-[calc(100%-2rem)]" onClick={() => void query.fetchNextPage()}>Load more</Button>}
        </div>
      )}
    </div>
  );
}
