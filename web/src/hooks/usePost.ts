import { useMutation, useQuery, useQueryClient, useInfiniteQuery } from '@tanstack/react-query';
import {
  bookmarkPost,
  createBookmarkCategory,
  createPost,
  getBookmarkedPosts,
  getBookmarkCategories,
  getFeedPosts,
  updatePost,
  deletePost,
  pinPost,
  unpinPost,
  getPostEdits,
  votePoll,
  getPost,
  getUserPosts,
  likePost,
  quotePost,
  repostPost,
  unbookmarkPost,
  unlikePost,
  unrepostPost,
} from '@/api/posts';
import type {
  BookmarkActionResponse,
  BookmarkCategory,
  CreateBookmarkCategoryPayload,
  CreateBookmarkCategoryResponse,
  CreatePostPayload,
  Envelope,
  MediaItem,
  PaginatedFeedResponse,
  Post,
  PostWithAncestorsAndDescendants,
} from '@/types/api';

// Centralized cache management system
type PostUpdater = (post: Post) => Post;

// Common query keys that contain posts
const POST_QUERY_KEYS = ['feed', 'bookmarked', 'user-posts'] as const;

interface InfinitePages {
  pages: Envelope<PaginatedFeedResponse>[];
}

const updatePostInPages = (
  old: InfinitePages | undefined,
  postId: number,
  updater: PostUpdater
): InfinitePages | undefined => {
  if (!old) return old;
  return {
    ...old,
    pages: old.pages.map(page => ({
      ...page,
      data: {
        ...page.data,
        items: page.data.items.map(post => (post.id === postId ? updater(post) : post)),
      },
    })),
  };
};

const updatePostInAllQueries = (
  queryClient: ReturnType<typeof useQueryClient>,
  postId: number,
  updater: PostUpdater
) => {
  POST_QUERY_KEYS.forEach(queryKey => {
    queryClient.setQueriesData<InfinitePages>(
      { queryKey: [queryKey] },
      (old) => updatePostInPages(old, postId, updater)
    );
  });

  // Update single post
  queryClient.setQueriesData<Envelope<PostWithAncestorsAndDescendants>>(
    { queryKey: ['post', postId] },
    (old) => {
      if (!old) return old;
      return {
        ...old,
        data: {
          ...old.data,
          post: updater(old.data.post),
        },
      };
    }
  );
};

const updateAuthorInAllQueries = (
  queryClient: ReturnType<typeof useQueryClient>,
  username: string,
  updater: (author: Post['author']) => Post['author']
) => {
  const updateAuthorInQuery = (queryKey: string) => {
    queryClient.setQueriesData<InfinitePages>(
      { queryKey: [queryKey] },
      (old) => {
        if (!old) return old;
        return {
          ...old,
          pages: old.pages.map(page => ({
            ...page,
            data: {
              ...page.data,
              items: page.data.items.map(post =>
                post.author.username === username
                  ? { ...post, author: updater(post.author) }
                  : post
              ),
            },
          })),
        };
      }
    );
  };

  POST_QUERY_KEYS.forEach(updateAuthorInQuery);
  // Handle user-specific posts
  updateAuthorInQuery('user-posts');
};

type EngagementUpdates = Partial<Post['engagement']>;

// Numeric engagement fields are treated as deltas so like/unlike +1/-1 never
// clobber the true count (e.g. unlike at 0 must land at 0, not -1).
const applyEngagementMerge = (
  base: Post['engagement'],
  updates: EngagementUpdates
): Post['engagement'] => {
  const next = { ...base };
  (Object.entries(updates) as [keyof Post['engagement'], Post['engagement'][keyof Post['engagement']]][]).forEach(([key, value]) => {
    if ((key === 'like_count' || key === 'repost_count' || key === 'bookmark_count') && typeof value === 'number') {
      next[key] = Math.max(0, (base[key] as number) + value);
    } else {
      (next as Record<string, unknown>)[key] = value;
    }
  });
  return next;
};

// Common mutation pattern for post engagement
const useEngagementMutation = <TVariables>(
  mutationFn: (variables: TVariables) => Promise<Envelope<BookmarkActionResponse>>,
  getPostId: (variables: TVariables) => number,
  optimisticUpdate: (variables: TVariables) => EngagementUpdates,
  rollbackUpdate: (variables: TVariables) => EngagementUpdates
) => {
  const queryClient = useQueryClient();

  return useMutation<Envelope<BookmarkActionResponse>, Error, TVariables>({
    mutationFn,
    onMutate: async (variables) => {
      const postId = getPostId(variables);
      await queryClient.cancelQueries({ queryKey: ['post', postId] });
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: applyEngagementMerge(post.engagement, optimisticUpdate(variables)),
      }));
    },
    onSuccess: (_data, variables) => {
      // Refetch authoritative engagement state after the server confirms so
      // the UI reflects the real counts even if optimistic math drifted.
      const postId = getPostId(variables);
      void queryClient.invalidateQueries({ queryKey: ['post', postId] });
      void queryClient.invalidateQueries({ queryKey: ['feed'] });
      void queryClient.invalidateQueries({ queryKey: ['bookmarked'] });
      void queryClient.invalidateQueries({ queryKey: ['user-posts'] });
      void queryClient.invalidateQueries({ queryKey: ['search-posts'] });
      void queryClient.invalidateQueries({ queryKey: ['hashtag-posts'] });
    },
    onError: (_err, variables) => {
      const postId = getPostId(variables);
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: applyEngagementMerge(post.engagement, rollbackUpdate(variables)),
      }));
    },
  });
};

export function useCreatePost() {
  const queryClient = useQueryClient();

  return useMutation<Envelope<Post>, Error, CreatePostPayload>({
    mutationFn: createPost,
    onSuccess: (response, variables) => {
      // Add the new post to the beginning of the feed
      queryClient.setQueryData<InfinitePages>(
        ['feed'],
        (old) => {
          if (!old || !old.pages[0]) return old;
          return {
            ...old,
            pages: [
              {
                ...old.pages[0],
                data: {
                  ...old.pages[0].data,
                  items: [response.data, ...old.pages[0].data.items],
                },
              },
              ...old.pages.slice(1),
            ],
          };
        }
      );
      // A reply must refresh the parent post page (ancestors + descendants).
      if (variables.parent_id != null) {
        void queryClient.invalidateQueries({ queryKey: ['post', variables.parent_id] });
      }
      // New content can show up in profile / search / hashtag feeds.
      void queryClient.invalidateQueries({ queryKey: ['user-posts'] });
      void queryClient.invalidateQueries({ queryKey: ['search-posts'] });
      void queryClient.invalidateQueries({ queryKey: ['hashtag-posts'] });
    },
  });
}

export function useGetFeedPosts(limit: number = 20) {
  return useInfiniteQuery({
    queryKey: ['feed'],
    queryFn: ({ pageParam }) => getFeedPosts(pageParam, limit),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.data.next_cursor,
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

function invalidatePostQueries(queryClient: ReturnType<typeof useQueryClient>, postId: number, username?: string) {
  void queryClient.invalidateQueries({ queryKey: ['post', postId] });
  void queryClient.invalidateQueries({ queryKey: ['feed'] });
  void queryClient.invalidateQueries({ queryKey: ['user-posts'] });
  void queryClient.invalidateQueries({ queryKey: ['search-posts'] });
  void queryClient.invalidateQueries({ queryKey: ['hashtag-posts'] });
  if (username) {
    void queryClient.invalidateQueries({ queryKey: ['pinned-post', username] });
  }
}

export function useUpdatePost() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ postId, content }: { postId: number; content: string; username?: string }) => updatePost(postId, content),
    onSuccess: (_, variables) => invalidatePostQueries(queryClient, variables.postId, variables.username),
  });
}

export function useDeletePost() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ postId }: { postId: number; username?: string }) => deletePost(postId),
    onSuccess: (_, variables) => {
      // Deleting the currently pinned post must clear the pinned-post cache
      // entry. Relying on a refetch here does not work: React Query keeps the
      // last successful data when the follow-up refetch 404s.
      if (variables.username) {
        queryClient.removeQueries({ queryKey: ['pinned-post', variables.username] });
      }
      invalidatePostQueries(queryClient, variables.postId, variables.username);
    },
  });
}

export function usePinPost() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ postId, pinned }: { postId: number; pinned: boolean; username?: string }) => pinned ? unpinPost(postId) : pinPost(postId),
    onSuccess: (_, variables) => {
      if (variables.username) {
        if (variables.pinned) {
          // Unpinned: the pinned-post query now 404s, but React Query retains
          // the previous data on a failed refetch, so purge the cache instead
          // of relying on invalidation.
          queryClient.removeQueries({ queryKey: ['pinned-post', variables.username] });
        } else {
          void queryClient.invalidateQueries({ queryKey: ['pinned-post', variables.username] });
        }
      }
      invalidatePostQueries(queryClient, variables.postId, variables.username);
    },
  });
}

export function usePostEdits(postId: number, enabled: boolean) {
  return useQuery({ queryKey: ['post-edits', postId], queryFn: () => getPostEdits(postId), enabled });
}

export function useVotePoll() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ postId, optionId }: { postId: number; optionId: number }) => votePoll(postId, optionId),
    onSuccess: (_, variables) => invalidatePostQueries(queryClient, variables.postId),
  });
}

export function useLikePost() {
  return useEngagementMutation(
    likePost,
    (postId) => postId,
    () => ({ is_liked: true, like_count: 1 }),
    () => ({ is_liked: false, like_count: -1 })
  );
}

export function useUnlikePost() {
  return useEngagementMutation(
    unlikePost,
    (postId) => postId,
    () => ({ is_liked: false, like_count: -1 }),
    () => ({ is_liked: true, like_count: 1 })
  );
}

export function useRepostPost() {
  return useEngagementMutation(
    repostPost,
    (postId) => postId,
    () => ({ is_reposted: true, repost_count: 1 }),
    () => ({ is_reposted: false, repost_count: -1 })
  );
}

export function useUnrepostPost() {
  return useEngagementMutation(
    unrepostPost,
    (postId) => postId,
    () => ({ is_reposted: false, repost_count: -1 }),
    () => ({ is_reposted: true, repost_count: 1 })
  );
}

// Bookmark mutations need to keep the bookmarks page and the category filter
// badges in sync, so they bypass the generic engagement mutation.
const invalidateBookmarkQueries = (
  queryClient: ReturnType<typeof useQueryClient>,
  postId: number
) => {
  void queryClient.invalidateQueries({ queryKey: ['post', postId] });
  void queryClient.invalidateQueries({ queryKey: ['feed'] });
  void queryClient.invalidateQueries({ queryKey: ['bookmarked'] });
  void queryClient.invalidateQueries({ queryKey: ['user-posts'] });
  void queryClient.invalidateQueries({ queryKey: ['search-posts'] });
  void queryClient.invalidateQueries({ queryKey: ['hashtag-posts'] });
  void queryClient.invalidateQueries({ queryKey: ['bookmark-categories'] });
};

// Optimistically drop the post from every bookmarked feed. Used on unbookmark
// so the bookmarks page reacts immediately instead of waiting for the refetch.
const removePostFromBookmarkedFeeds = (
  queryClient: ReturnType<typeof useQueryClient>,
  postId: number
) => {
  queryClient.setQueriesData<InfinitePages>(
    { queryKey: ['bookmarked'] },
    (old) => {
      if (!old) return old;
      return {
        ...old,
        pages: old.pages.map(page => ({
          ...page,
          data: {
            ...page.data,
            items: page.data.items.filter(post => post.id !== postId),
          },
        })),
      };
    }
  );
};

export function useBookmarkPost() {
  const queryClient = useQueryClient();

  return useMutation<Envelope<BookmarkActionResponse>, Error, { postId: number; categoryId?: number }>({
    mutationFn: ({ postId, categoryId }) => bookmarkPost(postId, categoryId),
    onMutate: async ({ postId }) => {
      await queryClient.cancelQueries({ queryKey: ['post', postId] });
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: applyEngagementMerge(post.engagement, {
          is_bookmarked: true,
          bookmark_count: post.engagement.is_bookmarked ? 0 : 1,
        }),
      }));
    },
    onSuccess: (_data, { postId }) => invalidateBookmarkQueries(queryClient, postId),
    onError: (_err, { postId }) => {
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: applyEngagementMerge(post.engagement, {
          is_bookmarked: false,
          bookmark_count: post.engagement.is_bookmarked ? -1 : 0,
        }),
      }));
    },
  });
}

export function useUnbookmarkPost() {
  const queryClient = useQueryClient();

  return useMutation<Envelope<BookmarkActionResponse>, Error, number>({
    mutationFn: unbookmarkPost,
    onMutate: async (postId) => {
      await queryClient.cancelQueries({ queryKey: ['post', postId] });
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: applyEngagementMerge(post.engagement, {
          is_bookmarked: false,
          bookmark_count: post.engagement.is_bookmarked ? -1 : 0,
        }),
      }));
      removePostFromBookmarkedFeeds(queryClient, postId);
    },
    onSuccess: (_data, postId) => invalidateBookmarkQueries(queryClient, postId),
    onError: (_err, postId) => {
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: applyEngagementMerge(post.engagement, {
          is_bookmarked: true,
          bookmark_count: post.engagement.is_bookmarked ? 0 : 1,
        }),
      }));
    },
  });
}

export function useQuotePost(postId: number) {
  const queryClient = useQueryClient();

  return useMutation<Envelope<Post>, Error, { content: string; media: MediaItem[]; parent_id: number | null }>({
    mutationFn: (payload) => quotePost(postId, payload),
    onSuccess: (response) => {
      // Add the quote to the beginning of the feed
      queryClient.setQueryData<InfinitePages>(
        ['feed'],
        (old) => {
          if (!old || !old.pages[0]) return old;
          return {
            ...old,
            pages: [
              {
                ...old.pages[0],
                data: {
                  ...old.pages[0].data,
                  items: [response.data, ...old.pages[0].data.items],
                },
              },
              ...old.pages.slice(1),
            ],
          };
        }
      );
      // Refresh the quoted post's engagement (quote count)
      queryClient.invalidateQueries({ queryKey: ['post', postId] });
    },
  });
}

export function useGetBookmarkedPosts(categoryIds?: number[], limit: number = 10) {
  return useInfiniteQuery({
    queryKey: ['bookmarked', categoryIds && categoryIds.length > 0 ? categoryIds.join(',') : 'all'],
    queryFn: ({ pageParam }) => getBookmarkedPosts(categoryIds, pageParam, limit),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.data.next_cursor,
  });
}

export function useGetBookmarkCategories() {
  return useQuery<Envelope<BookmarkCategory[]>, Error>({
    queryKey: ['bookmark-categories'],
    queryFn: getBookmarkCategories,
  });
}

export function useCreateBookmarkCategory() {
  const queryClient = useQueryClient();

  return useMutation<Envelope<CreateBookmarkCategoryResponse>, Error, CreateBookmarkCategoryPayload>({
    mutationFn: createBookmarkCategory,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bookmark-categories'] });
    },
  });
}

export function useGetUserPosts(username: string, limit: number = 20) {
  return useInfiniteQuery({
    queryKey: ['user-posts', username],
    queryFn: ({ pageParam }) => getUserPosts(username, pageParam, limit),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.data.next_cursor,
  });
}

export function updateAuthorInPostQueries(
  queryClient: ReturnType<typeof useQueryClient>,
  username: string,
  updates: { display_name?: string; profile_picture_uuid?: string }
) {
  updateAuthorInAllQueries(queryClient, username, (author) => ({
    ...author,
    display_name: updates.display_name ?? author.display_name,
    profile_picture_uuid: updates.profile_picture_uuid ?? author.profile_picture_uuid,
  }));
}

export function useGetPost(postId: number) {
  return useQuery<Envelope<PostWithAncestorsAndDescendants>, Error>({
    queryKey: ['post', postId],
    queryFn: () => getPost(postId),
  });
}
