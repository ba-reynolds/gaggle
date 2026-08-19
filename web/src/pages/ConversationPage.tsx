import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useConversation, useConversationMessages, useMarkConversationRead, useSendMessage } from '@/hooks/useDms';
import { useUser } from '@/contexts/UserContext';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ArrowLeft, Loader2, Send } from 'lucide-react';
import { getMediaUrl } from '@/util/media';
import { formatMessageDayLabel, formatMessageHour, getMessageDayKey } from '@/util/date';
import { toast } from 'sonner';

export default function ConversationPage() {
  const { conversationId: conversationIdStr } = useParams();
  const conversationId = Number(conversationIdStr);
  const { user } = useUser();
  const conversation = useConversation(conversationId);
  const messages = useConversationMessages(conversationId, 30);
  const markRead = useMarkConversationRead();
  const send = useSendMessage();

  const [body, setBody] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);

  const allMessages = useMemo(
    () => [...(messages.data?.pages ?? [])].flatMap((page) => page.data.items),
    [messages.data]
  );
  // API returns newest-first; display chronologically (oldest top, newest bottom).
  const displayMessages = useMemo(() => [...allMessages].reverse(), [allMessages]);

  // Mark the conversation read when opened.
  useEffect(() => {
    if (conversationId > 0) {
      markRead.mutate(conversationId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conversationId]);

  // Scroll to bottom on load and when a new (own) message is added.
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [allMessages.length]);

  const conv = conversation.data?.data;
  const participant = conv;

  const sendMessage = () => {
    if (!participant || !body.trim()) return;
    send.mutate(
      { username: participant.other_participant.username, body: body.trim() },
      {
        onSuccess: () => {
          setBody('');
          setTimeout(() => scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight }), 50);
        },
        onError: () => toast.error('Could not send message'),
      }
    );
  };

  if (conversation.isLoading) {
    return <div className="flex justify-center py-20"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  }
  if (conversation.isError || !conv) {
    return <div className="py-20 text-center text-muted-foreground">Conversation not found.</div>;
  }

  const other = conv.other_participant;

  return (
    <div className="mx-auto flex h-full w-full max-w-xl flex-col pt-2">
      <header className="border-b border-border px-4 py-3 flex items-center gap-3">
        <Link to="/messages" className="text-muted-foreground hover:text-primary"><ArrowLeft className="h-5 w-5" /></Link>
        <Link to={`/profile/${other.username}`} className="flex items-center gap-2">
          <Avatar className="h-9 w-9">
            <AvatarImage src={getMediaUrl(other.profile_picture_uuid)} alt={other.display_name} />
            <AvatarFallback>{other.display_name?.charAt(0) ?? '?'}</AvatarFallback>
          </Avatar>
          <span className="font-semibold text-primary">{other.display_name || other.username}</span>
        </Link>
      </header>

      <div ref={scrollRef} className="flex-1 overflow-y-auto space-y-2 p-4">
        {messages.isLoading ? (
          <div className="flex justify-center py-10"><Loader2 className="h-6 w-6 animate-spin" /></div>
        ) : (
          <>
            {messages.hasNextPage && (
              <Button variant="ghost" className="w-full text-sm" onClick={() => void messages.fetchNextPage()}>Load older</Button>
            )}
            {displayMessages.map((m, index) => {
              const mine = m.sender.username === user.username;
              const prev = displayMessages[index - 1];
              const isNewDay = !prev || getMessageDayKey(m.created_at) !== getMessageDayKey(prev.created_at);
              return (
                <div key={m.id}>
                  {isNewDay && (
                    <div className="my-3 flex items-center gap-3">
                      <div className="h-px flex-1 bg-border" />
                      <span className="rounded-full border border-border bg-muted px-3 py-1 text-xs text-muted-foreground">
                        {formatMessageDayLabel(m.created_at)}
                      </span>
                      <div className="h-px flex-1 bg-border" />
                    </div>
                  )}
                  <div className={`flex ${mine ? 'justify-end' : 'justify-start'}`}>
                    <div className={`min-w-0 max-w-[75%] rounded-2xl px-3 py-2 text-sm ${mine ? 'chat-bubble-mine' : 'chat-bubble-theirs text-primary'}`}>
                      {!mine && (
                        <p className="mb-0.5 text-xs text-muted-foreground">@{m.sender.username}</p>
                      )}
                      <p className="whitespace-pre-wrap break-words">{m.body}</p>
                      <p className={`mt-1 text-right text-[10px] ${mine ? 'text-white/70' : 'text-muted-foreground'}`}>
                        {formatMessageHour(m.created_at)}
                      </p>
                    </div>
                  </div>
                </div>
              );
            })}
          </>
        )}
      </div>

      <div className="border-t border-border p-3 flex gap-2">
        <Input
          value={body}
          onChange={(e) => setBody(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
          placeholder={`Message @${other.username}`}
          maxLength={2000}
          className="flex-1"
        />
        <Button onClick={sendMessage} disabled={!body.trim() || send.isPending}>
          <Send className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}