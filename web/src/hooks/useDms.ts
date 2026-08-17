import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getConversation,
  getConversationMessages,
  getConversations,
  getDmUnreadCount,
  markConversationRead,
  sendMessage,
} from '@/api/dms';
import type { SendMessagePayload } from '@/types/api';

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

export const useDmUnreadCount = () =>
  useQuery({ queryKey: ['dm-unread-count'], queryFn: getDmUnreadCount });

export const useSendMessage = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ username, body }: { username: string; body: string }) => sendMessage(username, { body } as SendMessagePayload),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: ['dm-conversations'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-unread-count'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-messages'] });
      void queryClient.invalidateQueries({ queryKey: ['dm-sent-conversation', variables.username] });
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