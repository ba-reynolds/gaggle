import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useConversations, useSentConversationForUser, useSendMessage } from '@/hooks/useDms';
import { useSearchUsers } from '@/hooks/useSearch';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Loader2, Mail, Send } from 'lucide-react';
import { getMediaUrl } from '@/util/media';
import { toast } from 'sonner';

export default function MessagesPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const toUser = params.get('user');
  const { data, isLoading } = useConversations();
  const conversations = useMemo(() => data?.data?.items ?? [], [data]);
  const existing = useSentConversationForUser(toUser ?? '');

  // When opened with ?user=X and we already have a conversation, go straight in.
  useEffect(() => {
    if (toUser && existing) {
      navigate(`/messages/${existing.id}`, { replace: true });
    }
  }, [toUser, existing, navigate]);

  // Pre-filled composer when a ?user target was provided (first contact).
  const [composerOpen] = useState(() => Boolean(toUser && !existing));
  const [composerText, setComposerText] = useState('');
  const sendMessage = useSendMessage();

  const handleNewMessage = () => {
    if (!toUser || !composerText.trim()) return;
    sendMessage.mutate(
      { username: toUser, body: composerText.trim() },
      {
        onSuccess: (response) => {
          navigate(`/messages/${response.data.conversation_id}`);
        },
        onError: () => toast.error('Could not send message'),
      }
    );
  };

  return (
    <div className="mx-auto w-full max-w-xl pt-6">
      <header className="px-4 pb-4 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-primary">Messages</h1>
      </header>

      <NewMessageComposer onSent={(convId) => navigate(`/messages/${convId}`)} />

      {composerOpen && (
        <div className="mx-4 mb-4 rounded-xl border border-border p-4">
          <p className="text-sm text-primary mb-2">Message @{toUser}</p>
          <div className="flex gap-2">
            <Input
              value={composerText}
              onChange={(e) => setComposerText(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleNewMessage()}
              placeholder="Write a message..."
              maxLength={2000}
            />
            <Button onClick={handleNewMessage} disabled={!composerText.trim() || sendMessage.isPending}>
              <Send className="h-4 w-4" />
            </Button>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
      ) : conversations.length === 0 && !composerOpen ? (
        <div className="flex flex-col items-center gap-3 p-12 text-center text-muted-foreground">
          <Mail className="h-10 w-10" />
          <p>No conversations yet.</p>
          <p className="text-sm">Find someone and hit Message on their profile.</p>
        </div>
      ) : (
        <div className="space-y-2 px-4 pb-8">
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

function NewMessageComposer({ onSent }: { onSent: (convId: number) => void }) {
  const [query, setQuery] = useState('');
  const results = useSearchUsers(query);
  const sendMessage = useSendMessage();

  const pick = (username: string, body?: string) => {
    sendMessage.mutate(
      { username, body: body?.trim() || 'Hello!' },
      {
        onSuccess: (response) => onSent(response.data.conversation_id),
        onError: () => toast.error('Could not start a conversation'),
      }
    );
  };

  return (
    <div className="mx-4 mb-4">
      <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search users to message..." />
      {query && (
        <div className="mt-2 space-y-1">
          {results.data?.data.items.slice(0, 5).map((u) => (
            <button
              key={u.username}
              className="flex w-full items-center gap-3 rounded-lg border border-border p-2 text-left hover:bg-muted"
              onClick={() => pick(u.username)}
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