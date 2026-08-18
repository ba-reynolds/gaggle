import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  addUserToList,
  createList,
  deleteList,
  getList,
  getListFeed,
  getListMembers,
  getMyLists,
  getUserLists,
  removeUserFromList,
  updateList,
} from '@/api/lists';
import type { CreateListPayload, List } from '@/types/api';

export const useMyLists = () =>
  useQuery({ queryKey: ['my-lists'], queryFn: getMyLists });

export const useUserLists = (username: string) =>
  useQuery({ queryKey: ['user-lists', username], queryFn: () => (username ? getUserLists(username) : Promise.resolve({ data: [], error: null as never })), enabled: !!username });

export const useList = (listId: number) =>
  useQuery({ queryKey: ['list', listId], queryFn: () => getList(listId), enabled: listId > 0 });

export const useListFeed = (listId: number, limit: number = 20) =>
  useInfiniteQuery({
    queryKey: ['list-feed', listId],
    queryFn: ({ pageParam }) => getListFeed(listId, pageParam, limit),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.data.next_cursor ?? undefined,
    enabled: listId > 0,
  });

export const useListMembers = (listId: number, limit: number = 50) =>
  useInfiniteQuery({
    queryKey: ['list-members', listId],
    queryFn: ({ pageParam }) => getListMembers(listId, pageParam, limit),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.data.next_cursor ?? undefined,
    enabled: listId > 0,
  });

const invalidateListQueries = (queryClient: ReturnType<typeof useQueryClient>, listId?: number) => {
  void queryClient.invalidateQueries({ queryKey: ['my-lists'] });
  void queryClient.invalidateQueries({ queryKey: ['user-lists'] });
  if (listId) {
    void queryClient.invalidateQueries({ queryKey: ['list', listId] });
    void queryClient.invalidateQueries({ queryKey: ['list-feed', listId] });
    void queryClient.invalidateQueries({ queryKey: ['list-members', listId] });
  }
};

export const useCreateList = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateListPayload) => createList(payload),
    onSuccess: () => invalidateListQueries(queryClient),
  });
};

export const useDeleteList = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (listId: number) => deleteList(listId),
    onSuccess: (_, listId) => invalidateListQueries(queryClient, listId),
  });
};

export const useUpdateList = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ listId, payload }: { listId: number; payload: CreateListPayload }) => updateList(listId, payload),
    onSuccess: (_, { listId }) => invalidateListQueries(queryClient, listId),
  });
};

export const useAddUserToList = (listId: number) => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (username: string) => addUserToList(listId, username),
    onSuccess: () => invalidateListQueries(queryClient, listId),
  });
};

export const useAddUsersToList = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ listId, usernames }: { listId: number; usernames: string[] }) =>
      Promise.all(usernames.map((username) => addUserToList(listId, username))),
    onSuccess: (_, { listId }) => invalidateListQueries(queryClient, listId),
  });
};

export const useRemoveUserFromList = (listId: number) => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (username: string) => removeUserFromList(listId, username),
    onSuccess: () => invalidateListQueries(queryClient, listId),
  });
};

export type { List };