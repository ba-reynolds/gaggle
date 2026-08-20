import { useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import FeedPost from '@/components/FeedPost';
import UserHoverCard from '@/components/UserHoverCard';
import { UserAvatar } from '@/components/UserAvatar';
import { Button } from '@/components/ui/button';
import { useList, useListFeed, useListMembers, useAddUserToList, useRemoveUserFromList } from '@/hooks/useLists';
import { useSearchUsers } from '@/hooks/useSearch';
import { SEARCH_DEBOUNCE_MS, useDebounce } from '@/hooks/useDebounce';
import { useUser } from '@/contexts/UserContext';
import { getMediaUrl } from '@/util/media';
import { List as ListIcon, Loader2, UserPlus, UserMinus, Users } from 'lucide-react';
import { toast } from 'sonner';

export default function ListPage() {
  const { id } = useParams();
  const listId = Number(id);
  const { user } = useUser();

  const listQuery = useList(listId);
  const feed = useListFeed(listId);
  const members = useListMembers(listId);
  const addMember = useAddUserToList(listId);
  const removeMember = useRemoveUserFromList(listId);

  const list = listQuery.data?.data;
  const isOwner = user.username !== '' && list?.owner_username === user.username;
  const posts = useMemo(() => feed.data?.pages.flatMap((page) => page.data.items) ?? [], [feed.data]);
  const memberItems = useMemo(() => members.data?.pages.flatMap((page) => page.data.items) ?? [], [members.data]);

  const toggleMember = (username: string) => {
    if (memberItems.some((m) => m.username === username)) {
      removeMember.mutate(username, {
        onSuccess: () => toast.success(`Removed @${username}`),
        onError: () => toast.error('Failed to remove member'),
      });
    } else {
      addMember.mutate(username, {
        onSuccess: () => toast.success(`Added @${username}`),
        onError: (e: Error) => toast.error(e.message || 'Failed to add member'),
      });
    }
  };

  if (listQuery.isLoading) {
    return <div className="flex justify-center py-20"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  }
  if (listQuery.isError || !list) {
    return <div className="py-20 text-center text-muted-foreground">List not found.</div>;
  }

  return (
    <div className="mx-auto w-full max-w-xl">
      <header className="border-b border-border p-5">
        <div className="flex items-center gap-2">
          <ListIcon className="h-6 w-6 text-primary" />
          <h1 className="text-2xl font-bold text-primary">{list.name}</h1>
        </div>
        <p className="text-sm text-muted-foreground mt-1">
          by <Link to={`/profile/${list.owner_username}`} className="hover:underline text-primary">@{list.owner_username}</Link>
        </p>
        {list.description && <p className="text-sm text-muted-foreground mt-2">{list.description}</p>}
        <p className="text-xs text-muted-foreground mt-2 flex items-center gap-1"><Users className="h-3 w-3" />{list.member_count} members</p>
      </header>

      <div className="space-y-4 p-4">
        {feed.isLoading && <Loader2 className="mx-auto mt-8 h-8 w-8 animate-spin text-primary" />}
        {!feed.isLoading && posts.length === 0 && <p className="p-8 text-center text-muted-foreground">No posts from list members yet.</p>}
        {posts.map((post) => <FeedPost key={post.id} post={post} />)}
        {feed.hasNextPage && (
          <Button variant="outline" className="w-full" onClick={() => void feed.fetchNextPage()}>Load more</Button>
        )}
      </div>

      <div className="border-t border-border px-4 py-6">
        <h2 className="mb-3 text-lg font-semibold text-primary">Members</h2>
        {memberItems.length === 0 ? (
          <p className="text-sm text-muted-foreground">No members yet.</p>
        ) : (
          <div className="space-y-2">
            {memberItems.map((member) => (
              <div key={member.username} className="flex items-center justify-between rounded-xl border border-border p-3">
                <UserHoverCard
                  name={member.display_name}
                  username={member.username}
                  userDescription={member.bio}
                  followers={member.followers_count}
                  following={member.following_count}
                >
                  <div className="flex min-w-0 items-center">
                    <UserAvatar className="mr-2 h-10 w-10 shrink-0" src={getMediaUrl(member.profile_picture_uuid)} name={member.display_name} username={member.username} />
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-primary">{member.display_name || member.username}</p>
                      <p className="truncate text-xs text-muted-foreground">@{member.username}</p>
                    </div>
                  </div>
                </UserHoverCard>
                {isOwner && (
                  <Button size="sm" variant="ghost" onClick={() => toggleMember(member.username)}>
                    <UserMinus className="h-4 w-4" />
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}

        {isOwner && (
          <div className="mt-4">
            <p className="text-xs uppercase tracking-wider text-muted-foreground mb-2">Add a user</p>
            <MemberSearch listId={listId} onAdd={toggleMember} />
          </div>
        )}
      </div>
    </div>
  );
}

function MemberSearch({ onAdd }: { listId: number; onAdd: (username: string) => void }) {
  const [query, setQuery] = useState('');
  const debouncedQuery = useDebounce(query, SEARCH_DEBOUNCE_MS);
  const results = useSearchUsers(debouncedQuery);
  return (
    <div>
      <input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search users to add..."
        className="w-full rounded-lg border border-border bg-transparent px-3 py-2 text-sm text-primary"
      />
      {debouncedQuery && (
        <div className="space-y-1 mt-2">
          {results.data?.data.items.slice(0, 5).map((u) => (
            <div key={u.username} className="flex items-center justify-between rounded-lg border border-border p-2">
              <div className="flex items-center">
                <UserAvatar className="h-8 w-8 mr-2" src={getMediaUrl(u.profile_picture_uuid)} name={u.display_name} username={u.username} />
                <div>
                  <p className="text-sm font-medium text-primary">{u.display_name || u.username}</p>
                  <p className="text-xs text-muted-foreground">@{u.username}</p>
                </div>
              </div>
              <Button size="sm" variant="outline" onClick={() => onAdd(u.username)}><UserPlus className="h-4 w-4" /></Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}