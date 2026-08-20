import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getConversation,
  getConversationMessages,
  getConversations,
  getDmUnreadCount,
  markConversationRead,
  sendMessage,
} from '@/api/dms';
import type { Envelope, Message, MessageSender, PaginatedFeedResponse, SendMessagePayload } from '@/types/api';

export const useConversations = () =>
  useQuery({ queryKey: ['dm-conversations'], queryFn: getConversations });

export const useConversation = (conversationId: number) =>
  useQuery({
    queryKey: ['dm-conversation', conversationId],
    queryFn: () => getConversation(conversationId),
    enabled: conversationId > 0,
  });

export const useConversationMessages = (conversationId: number, limit: number = 30) =>
  useInfiniteQuery({
    queryKey: ['dm-messages', conversationId],
    queryFn: ({ pageParam }) => getConversationMessages(conversationId, pageParam, limit),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.data.next_cursor ?? undefined,
    enabled: conversationId > 0,
  });

export const useDmUnreadCount = (enabled: boolean = true) =>
  useQuery({ queryKey: ['dm-unread-count'], queryFn: getDmUnreadCount, enabled });

type MessagePages = Envelope<PaginatedFeedResponse<Message>>[];

interface SendMessageVariables {
  username: string;
  body: string;
  conversationId?: number;
  sender: MessageSender;
}

// Inserts a temporary message into the conversation's cached pages so it
// renders immediately; swapped for the server-confirmed message on success.
const insertTempMessage = (
  queryClient: ReturnType<typeof useQueryClient>,
  conversationId: number,
  tempMessage: Message
) => {
  queryClient.setQueryData(
    ['dm-messages', conversationId],
    (old: { pages: MessagePages } | undefined) => {
      if (!old || !old.pages[0]) return old;
      return {
        ...old,
        pages: [
          {
            ...old.pages[0],
            data: {
              ...old.pages[0].data,
              items: [tempMessage, ...old.pages[0].data.items],
            },
          },
          ...old.pages.slice(1),
        ],
      };
    }
  );
};

export const useSendMessage = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (variables: SendMessageVariables) =>
      sendMessage(variables.username, { body: variables.body } as SendMessagePayload),
    onMutate: async ({ conversationId, body, sender }) => {
      if (!conversationId) return undefined;

      await queryClient.cancelQueries({ queryKey: ['dm-messages', conversationId] });
      const previous = queryClient.getQueryData<{ pages: MessagePages }>(['dm-messages', conversationId]);

      const tempMessage: Message = {
        id: Date.now() * -1,
        conversation_id: conversationId,
        sender_id: 0,
        sender: { ...sender },
        body,
        created_at: new Date().toISOString(),
        pending: true,
      };
      insertTempMessage(queryClient, conversationId, tempMessage);

      return { previous, conversationId, tempId: tempMessage.id };
    },
    onSuccess: (_data, variables, context) => {
      if (context) {
        // Swap the optimistic placeholder for the server-confirmed message so
        // the conversation list and read state stay accurate without a flicker.
        queryClient.setQueryData(
          ['dm-messages', context.conversationId],
          (old: { pages: MessagePages } | undefined) => {
            if (!old) return old;
            return {
              ...old,
              pages: old.pages.map((page) => ({
                ...page,
                data: {
                  ...page.data,
                  items: page.data.items.map((m) => (m.id === context!.tempId ? _data.data : m)),
                },
              })),
            };
          }
        );
      }
      void queryClient.invalidateQueries({ queryKey: ['dm-conversations'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-unread-count'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-messages'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-sent-conversation', variables.username] });
    },
    onError: (_err, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['dm-messages', context.conversationId], context.previous);
      }
      void queryClient.invalidateQueries({ queryKey: ['dm-conversations'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-unread-count'] });
    },
  });
};

export const useMarkConversationRead = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: number) => markConversationRead(conversationId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['dm-conversations'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-unread-count'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-messages'] });
    },
  });
};

export const useSentConversationForUser = (username: string) => {
  const { data } = useConversations();
  const conversations = data?.data?.items ?? [];
  return conversations.find((c) => c.other_participant.username === username);
};