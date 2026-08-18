import { getHashtagPosts, getSuggestedUsers, getTrends, searchPosts, searchUsers } from '@/api/search';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';

export function useSearchPosts(query: string) {
  return useInfiniteQuery({
    queryKey: ['search-posts', query],
    queryFn: ({ pageParam }) => searchPosts(query, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.data.next_cursor ?? undefined,
    enabled: query.trim().length > 0,
  });
}

export function useSearchUsers(query: string) {
  return useQuery({
    queryKey: ['search-users', query],
    queryFn: () => searchUsers(query),
    enabled: query.trim().length > 0,
  });
}

export function useHashtagPosts(tag: string) {
  return useInfiniteQuery({
    queryKey: ['hashtag-posts', tag],
    queryFn: ({ pageParam }) => getHashtagPosts(tag, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.data.next_cursor ?? undefined,
    enabled: tag.trim().length > 0,
  });
}

export function useTrends(enabled: boolean = true) {
  return useQuery({ queryKey: ['trends'], queryFn: async () => (await getTrends()).data, enabled });
}

export function useSuggestedUsers(limit?: number, enabled: boolean = true) {
  return useQuery({ queryKey: ['suggested-users', limit], queryFn: async () => (await getSuggestedUsers(limit)).data, enabled });
}
