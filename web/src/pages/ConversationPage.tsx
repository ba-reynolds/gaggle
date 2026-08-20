import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useConversation, useConversationMessages, useMarkConversationRead, useSentConversationForUser, useSendMessage } from '@/hooks/useDms';
import { useUser } from '@/contexts/UserContext';
import { useFetchProfile } from '@/hooks/useUser';
import { UserAvatar } from '@/components/UserAvatar';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ArrowLeft, Loader2, Send } from 'lucide-react';
import { getMediaUrl } from '@/util/media';
import { formatMessageDayLabel, formatMessageHour, getMessageDayKey } from '@/util/date';
import { toast } from 'sonner';
import type { ConversationOtherParticipant, Message, UserProfileResponse } from '@/types/api';

// Messages from the same sender within this window (and same calendar day) are
// glued into a single bubble group that shows one timestamp, iMessage-style.
const MESSAGE_GROUP_WINDOW_MINUTES = 5;
const MESSAGE_GROUP_WINDOW_MS = MESSAGE_GROUP_WINDOW_MINUTES * 60_000;

const messagesAreClose = (a: Message, b: Message) =>
  a.sender.username === b.sender.username &&
  getMessageDayKey(a.created_at) === getMessageDayKey(b.created_at) &&
  Math.abs(Date.parse(a.created_at) - Date.parse(b.created_at)) <= MESSAGE_GROUP_WINDOW_MS;

// Message pages must NOT rely on the layout's column giving them a definite
// height: the column is `min-h-screen` (not `h-screen`) so the page background
// can grow past one viewport, meaning a child `h-full` resolves to auto and the
// thread would grow the document forever instead of scrolling internally. Pin
// the page to the viewport ourselves; on mobile the extra pb mirrors the
// column's `pb-16` so the fixed bottom nav doesn't cover the composer.
const MESSAGE_PAGE_HEIGHT = 'h-dvh pb-16 md:pb-0';

export default function ConversationPage() {
  const { conversationId: conversationIdStr } = useParams();
  const [params] = useSearchParams();

  // Opening a brand-new conversation (/messages/new?user=X) shows an empty chat
  // UI against the target's profile; the conversation is created on first send.
  // /messages/new is a STATIC route (no :conversationId param), so params has no
  // conversationId key there — the dynamic route only yields 'new' as a fallback.
  const isNew = !conversationIdStr || conversationIdStr === 'new';
  const targetUsername = isNew ? params.get('user') ?? '' : '';

  if (isNew) {
    return <NewConversationPage username={targetUsername} />;
  }

  const conversationId = Number(conversationIdStr);
  return <ExistingConversationPage conversationId={conversationId} />;
}

function ExistingConversationPage({ conversationId }: { conversationId: number }) {
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

  // Position of each message within its bubble group ('start'/'mid'/'end') or
  // 'standalone'. Drives corner rounding and single-timestamp-per-group.
  const groupPositions = useMemo(
    () =>
      displayMessages.map((m, index) => {
        const prev = displayMessages[index - 1];
        const next = displayMessages[index + 1];
        const closeToPrev = Boolean(prev && messagesAreClose(m, prev));
        const closeToNext = Boolean(next && messagesAreClose(m, next));
        if (closeToPrev && closeToNext) return 'mid';
        if (!closeToPrev && closeToNext) return 'start';
        if (closeToPrev && !closeToNext) return 'end';
        return 'standalone';
      }),
    [displayMessages]
  );

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
    const text = body.trim();
    setBody('');
    send.mutate(
      {
        username: participant.other_participant.username,
        body: text,
        conversationId,
        sender: {
          username: user.username,
          display_name: user.displayName,
          profile_picture_uuid: user.profilePictureUUID || undefined,
        },
      },
      {
        onError: () => {
          setBody(text);
          toast.error('Could not send message');
        },
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
    <div className={`mx-auto flex w-full max-w-xl flex-col pt-2 ${MESSAGE_PAGE_HEIGHT}`}>
      <header className="border-b border-border px-4 py-3 flex items-center gap-3">
        <Link to="/messages" className="text-muted-foreground hover:text-primary"><ArrowLeft className="h-5 w-5" /></Link>
        <ChatHeader user={other} />
      </header>

      <div ref={scrollRef} className="theme-scrollbar flex-1 overflow-y-auto p-4 min-h-0">
        {messages.isLoading ? (
          <div className="flex justify-center py-10"><Loader2 className="h-6 w-6 animate-spin" /></div>
        ) : (
          <>
            {messages.hasNextPage && (
              <Button variant="ghost" className="mb-2 w-full text-sm" onClick={() => void messages.fetchNextPage()}>Load older</Button>
            )}
            {displayMessages.map((m, index) => {
              const mine = m.sender.username === user.username;
              const prev = displayMessages[index - 1];
              const isNewDay = !prev || getMessageDayKey(m.created_at) !== getMessageDayKey(prev.created_at);
              const position = groupPositions[index];
              const isGroupStart = position === 'start' || position === 'standalone';
              const isGroupEnd = position === 'end' || position === 'standalone';
              return (
                <div key={m.id} className={isGroupEnd ? 'mb-3.5' : 'mb-0.5'}>
                  {isNewDay && (
                    <div className="my-3 flex items-center gap-3">
                      <div className="h-px flex-1 bg-border" />
                      <span className="rounded-full border border-border bg-muted px-3 py-1 text-xs text-muted-foreground">
                        {formatMessageDayLabel(m.created_at)}
                      </span>
                      <div className="h-px flex-1 bg-border" />
                    </div>
                  )}
                  {isGroupStart && (
                    <div className="my-2 text-center text-xs text-muted-foreground">
                      {m.pending ? 'Sending…' : formatMessageHour(m.created_at)}
                    </div>
                  )}
                  <div className={`flex ${mine ? 'justify-end' : 'justify-start'}`}>
                    <div className={`min-w-0 max-w-[75%] rounded-2xl px-3 py-2 text-sm ${position === 'standalone' ? '' : `group-${position}`} ${mine ? 'chat-bubble-mine' : 'chat-bubble-theirs text-primary'}`}>
                      {!mine && isGroupStart && (
                        <p className="mb-0.5 text-xs text-muted-foreground">@{m.sender.username}</p>
                      )}
                      <p className={`whitespace-pre-wrap break-words ${m.pending ? 'opacity-70' : ''}`}>{m.body}</p>
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

// NewConversationPage shows an empty chat UI against a target user who the
// current user hasn't spoken to yet. The conversation is created server-side on
// first send, at which point we switch to the real conversation route.
function NewConversationPage({ username }: { username: string }) {
  const navigate = useNavigate();
  const { user } = useUser();
  const send = useSendMessage();
  const [body, setBody] = useState('');

  // If a conversation with this user already exists, go straight in.
  const existing = useSentConversationForUser(username);
  useEffect(() => {
    if (username && existing) {
      navigate(`/messages/${existing.id}`, { replace: true });
    }
  }, [username, existing, navigate]);

  const { data: profileData, isLoading: profileLoading } = useFetchProfile(username, Boolean(username));
  const profile = profileData?.data;
  const blocked = Boolean(profile?.is_blocked);

  const sendMessage = () => {
    if (!profile || !body.trim()) return;
    const text = body.trim();
    setBody('');
    send.mutate(
      {
        username: profile.username,
        body: text,
        sender: {
          username: user.username,
          display_name: user.displayName,
          profile_picture_uuid: user.profilePictureUUID || undefined,
        },
      },
      {
        onSuccess: (response) => {
          navigate(`/messages/${response.data.conversation_id}`, { replace: true });
        },
        onError: () => {
          setBody(text);
          toast.error('Could not send message');
        },
      }
    );
  };

  if (!username) {
    return <div className="py-20 text-center text-muted-foreground">No user specified.</div>;
  }

  if (profileLoading) {
    return <div className="flex justify-center py-20"><Loader2 className="h-8 w-8 animate-spin text-primary" /></div>;
  }

  if (!profile) {
    return <div className="py-20 text-center text-muted-foreground">User not found.</div>;
  }

  return (
    <div className={`mx-auto flex w-full max-w-xl flex-col pt-2 ${MESSAGE_PAGE_HEIGHT}`}>
      <header className="border-b border-border px-4 py-3 flex items-center gap-3">
        <Link to="/messages" className="text-muted-foreground hover:text-primary"><ArrowLeft className="h-5 w-5" /></Link>
        <ChatHeader user={profile} />
      </header>

      <div className="theme-scrollbar flex-1 min-h-0 overflow-y-auto space-y-2 p-4">
        <div className="flex h-full min-h-0 flex-col items-center justify-center gap-3 p-8 text-center text-muted-foreground">
          <UserAvatar className="h-16 w-16" src={getMediaUrl(profile.profile_picture_uuid)} name={profile.display_name} username={profile.username} />
          <p className="font-semibold text-primary text-lg">{profile.display_name || profile.username}</p>
          {blocked ? (
            <p className="text-sm">You've blocked @{profile.username}, so you can't message them.</p>
          ) : (
            <p className="text-sm">You haven't talked to @{profile.username} yet. Say hello!</p>
          )}
        </div>
      </div>

      <div className="border-t border-border p-3 flex gap-2">
        <Input
          value={body}
          onChange={(e) => setBody(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && sendMessage()}
          placeholder={`Message @${profile.username}`}
          maxLength={2000}
          disabled={blocked}
          className="flex-1"
        />
        <Button onClick={sendMessage} disabled={!body.trim() || send.isPending || blocked}>
          <Send className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

function ChatHeader({ user }: { user: ConversationOtherParticipant | UserProfileResponse }) {
  return (
    <Link to={`/profile/${user.username}`} className="flex items-center gap-2">
      <UserAvatar className="h-9 w-9" src={getMediaUrl(user.profile_picture_uuid)} name={user.display_name} username={user.username} />
      <span className="font-semibold text-primary">{user.display_name || user.username}</span>
    </Link>
  );
}