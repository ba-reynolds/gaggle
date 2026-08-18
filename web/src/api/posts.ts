import api from '@/lib/api';
import type {
  BookmarkActionResponse,
  BookmarkCategory,
  CreateBookmarkCategoryPayload,
  CreateBookmarkCategoryResponse,
  CreatePostPayload,
  Envelope,
  PaginatedFeedResponse,
  Post,
  PostWithAncestorsAndDescendants,
  UserProfileResponse,
  PostEdit,
  Poll,
} from '@/types/api';

export const createPost = async (payload: CreatePostPayload): Promise<Envelope<Post>> => {
  const response = await api.post<Envelope<Post>>('/posts', payload);
  return response.data;
};

export const updatePost = async (postId: number, content: string): Promise<Envelope<Post>> => {
  const response = await api.patch<Envelope<Post>>(`/posts/${postId}`, { content });
  return response.data;
};

export const deletePost = async (postId: number): Promise<void> => {
  await api.delete(`/posts/${postId}`);
};

export const pinPost = async (postId: number): Promise<void> => {
  await api.post(`/posts/${postId}/pin`);
};

export const unpinPost = async (postId: number): Promise<void> => {
  await api.delete(`/posts/${postId}/pin`);
};

export const getPostEdits = async (postId: number): Promise<Envelope<{ items: PostEdit[] }>> => {
  const response = await api.get<Envelope<{ items: PostEdit[] }>>(`/posts/${postId}/edits`);
  return response.data;
};

export const votePoll = async (postId: number, optionId: number): Promise<Envelope<Poll>> => {
  const response = await api.post<Envelope<Poll>>(`/posts/${postId}/poll/vote`, { option_id: optionId });
  return response.data;
};

export const getFeedPosts = async (cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse>>('/posts/feed', {
    params: { limit, cursor },
  });
  return response.data;
};

export const getPost = async (
  postId: number,
  includeAncestors = true,
  includeDescendants = true,
  limit = 20,
): Promise<Envelope<PostWithAncestorsAndDescendants>> => {
  const response = await api.get<Envelope<PostWithAncestorsAndDescendants>>(`/posts/${postId}`, {
    params: {
      ancestors: includeAncestors,
      descendants: includeDescendants,
      limit,
    },
  });
  return response.data;
};

export const likePost = async (postId: number): Promise<Envelope<BookmarkActionResponse>> => {
  const response = await api.post<Envelope<BookmarkActionResponse>>(`/posts/${postId}/like`);
  return response.data;
};

export const unlikePost = async (postId: number): Promise<Envelope<BookmarkActionResponse>> => {
  const response = await api.delete<Envelope<BookmarkActionResponse>>(`/posts/${postId}/like`);
  return response.data;
};

export const repostPost = async (postId: number): Promise<Envelope<BookmarkActionResponse>> => {
  const response = await api.post<Envelope<BookmarkActionResponse>>(`/posts/${postId}/repost`);
  return response.data;
};

export const unrepostPost = async (postId: number): Promise<Envelope<BookmarkActionResponse>> => {
  const response = await api.delete<Envelope<BookmarkActionResponse>>(`/posts/${postId}/repost`);
  return response.data;
};

export const quotePost = async (postId: number, payload: CreatePostPayload): Promise<Envelope<Post>> => {
  const response = await api.post<Envelope<Post>>(`/posts/${postId}/quote`, payload);
  return response.data;
};

export const bookmarkPost = async (postId: number, categoryId?: number): Promise<Envelope<BookmarkActionResponse>> => {
  const response = await api.post<Envelope<BookmarkActionResponse>>(`/posts/${postId}/bookmark`, {
    category_id: categoryId,
  });
  return response.data;
};

export const unbookmarkPost = async (postId: number): Promise<Envelope<BookmarkActionResponse>> => {
  const response = await api.delete<Envelope<BookmarkActionResponse>>(`/posts/${postId}/bookmark`);
  return response.data;
};

export const getBookmarkedPosts = async (categoryIds?: number[], cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const params: Record<string, string | number | undefined> = { limit, cursor };
  if (categoryIds && categoryIds.length > 0) {
    params.category_ids = categoryIds.join(',');
  }
  const response = await api.get<Envelope<PaginatedFeedResponse>>('/posts/bookmarks', { params });
  return response.data;
};

export const getBookmarkCategories = async (): Promise<Envelope<BookmarkCategory[]>> => {
  const response = await api.get<Envelope<BookmarkCategory[]>>('/bookmarks/category');
  return response.data;
};

export const createBookmarkCategory = async (payload: CreateBookmarkCategoryPayload): Promise<Envelope<CreateBookmarkCategoryResponse>> => {
  const response = await api.post<Envelope<CreateBookmarkCategoryResponse>>('/bookmarks/category', payload);
  return response.data;
};

export const deleteBookmarkCategory = async (categoryId: number): Promise<void> => {
  await api.delete(`/bookmarks/category/${categoryId}`);
};

export const getUserPosts = async (username: string, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse>>(`/users/${username}/posts`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getUserReplies = async (username: string, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse>>(`/users/${username}/replies`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getUserMedia = async (username: string, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse>>(`/users/${username}/media`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getUserLikes = async (username: string, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse>>(`/users/${username}/likes`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getPostQuotes = async (postId: number, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse>>(`/posts/${postId}/quotes`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getPostLikers = async (postId: number, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse<UserProfileResponse>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<UserProfileResponse>>>(`/posts/${postId}/likers`, {
    params: { limit, cursor },
  });
  return response.data;
};

export const getPostReposters = async (postId: number, cursor?: string, limit?: number): Promise<Envelope<PaginatedFeedResponse<UserProfileResponse>>> => {
  const response = await api.get<Envelope<PaginatedFeedResponse<UserProfileResponse>>>(`/posts/${postId}/reposters`, {
    params: { limit, cursor },
  });
  return response.data;
};
