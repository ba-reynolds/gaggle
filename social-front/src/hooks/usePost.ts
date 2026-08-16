import { useMutation, useQuery, useQueryClient, useInfiniteQuery } from '@tanstack/react-query';
import {
  bookmarkPost,
  createBookmarkCategory,
  createPost,
  getBookmarkedPosts,
  getBookmarkCategories,
  getFeedPosts,
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
        engagement: {
          ...post.engagement,
          ...optimisticUpdate(variables),
        },
      }));
    },
    onError: (_err, variables) => {
      const postId = getPostId(variables);
      updatePostInAllQueries(queryClient, postId, (post) => ({
        ...post,
        engagement: {
          ...post.engagement,
          ...rollbackUpdate(variables),
        },
      }));
    },
  });
};

export function useCreatePost() {
  const queryClient = useQueryClient();

  return useMutation<Envelope<Post>, Error, CreatePostPayload>({
    mutationFn: createPost,
    onSuccess: (response) => {
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

export function useBookmarkPost() {
  return useEngagementMutation(
    ({ postId, categoryId }: { postId: number; categoryId?: number }) => bookmarkPost(postId, categoryId),
    ({ postId }) => postId,
    ({ categoryId }) => ({
      is_bookmarked: true,
      bookmark_count: categoryId ? 1 : 1,
    }),
    ({ categoryId }) => ({
      is_bookmarked: false,
      bookmark_count: categoryId ? -1 : -1,
    })
  );
}

export function useUnbookmarkPost() {
  return useEngagementMutation(
    unbookmarkPost,
    (postId) => postId,
    () => ({ is_bookmarked: false, bookmark_count: -1 }),
    () => ({ is_bookmarked: true, bookmark_count: 1 })
  );
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