import { useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useConversations } from '@/hooks/useDms';
import { useSearchUsers } from '@/hooks/useSearch';
import { SEARCH_DEBOUNCE_MS, useDebounce } from '@/hooks/useDebounce';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Input } from '@/components/ui/input';
import { Loader2, Mail } from 'lucide-react';
import { getMediaUrl } from '@/util/media';

export default function MessagesPage() {
  const navigate = useNavigate();
  const { data, isLoading } = useConversations();
  const conversations = useMemo(() => data?.data?.items ?? [], [data]);

  return (
    <div className="mx-auto flex h-full w-full max-w-xl flex-col pt-6">
      <header className="px-4 pb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-primary">Messages</h1>
      </header>

      <NewMessageComposer onPick={(username) => navigate(`/messages/new?user=${encodeURIComponent(username)}`)} />

      {isLoading ? (
        <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
      ) : conversations.length === 0 ? (
        <div className="flex flex-col items-center gap-3 p-12 text-center text-muted-foreground">
          <Mail className="h-10 w-10" />
          <p>No conversations yet.</p>
          <p className="text-sm">Find someone above and start a conversation.</p>
        </div>
      ) : (
        <div className="flex-1 min-h-0 overflow-y-auto space-y-2 px-4 pb-8">
          {conversations.map((conv) => (
            <Link
              key={conv.id}
              to={`/messages/${conv.id}`}
              className="flex items-center justify-between rounded-xl border border-border p-4 hover:bg-muted"
            >
              <div className="flex items-center min-w-0">
                <Avatar className="h-10 w-10 mr-3">
                  <AvatarImage src={getMediaUrl(conv.other_participant.profile_picture_uuid)} alt={conv.other_participant.display_name} />
                  <AvatarFallback>{conv.other_participant.display_name?.charAt(0) ?? '?'}</AvatarFallback>
                </Avatar>
                <div className="min-w-0">
                  <p className="font-semibold text-primary">{conv.other_participant.display_name || conv.other_participant.username}</p>
                  <p className="text-sm text-muted-foreground truncate">@{conv.other_participant.username}</p>
                  {conv.last_message && (
                    <p className="text-sm text-muted-foreground truncate mt-0.5">{conv.last_message.body}</p>
                  )}
                </div>
              </div>
              {conv.unread_count > 0 && (
                <span className="ml-3 h-5 min-w-5 rounded-full bg-primary px-1.5 text-xs leading-5 text-primary-foreground text-center">
                  {conv.unread_count}
                </span>
              )}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function NewMessageComposer({ onPick }: { onPick: (username: string) => void }) {
  const [query, setQuery] = useState('');
  const debouncedQuery = useDebounce(query, SEARCH_DEBOUNCE_MS);
  const results = useSearchUsers(debouncedQuery);

  return (
    <div className="mx-4 mb-4">
      <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search users to message..." />
      {debouncedQuery && (
        <div className="mt-2 space-y-1">
          {results.data?.data.items.slice(0, 5).map((u) => (
            <button
              key={u.username}
              className="flex w-full items-center gap-3 rounded-lg border border-border p-2 text-left hover:bg-muted"
              onClick={() => onPick(u.username)}
            >
              <Avatar className="h-8 w-8">
                <AvatarImage src={getMediaUrl(u.profile_picture_uuid)} />
                <AvatarFallback>{u.display_name?.[0] ?? '?'}</AvatarFallback>
              </Avatar>
              <span>
                <span className="block text-sm font-medium text-primary">{u.display_name || u.username}</span>
                <span className="block text-xs text-muted-foreground">@{u.username}</span>
              </span>
            </button>
          ))}
          {!results.isLoading && !results.data?.data.items.length && (
            <p className="text-sm text-muted-foreground p-2">No users found.</p>
          )}
        </div>
      )}
    </div>
  );
}