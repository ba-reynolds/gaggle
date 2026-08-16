// Response envelope type. Every endpoint returns this shape; the frontend
// must always read `response.data.data`.
export interface ApiError {
  code: string;
  message: string;
}

export interface Envelope<T> {
  data: T;
  error: ApiError | null;
}

// Auth
export interface LoginPayload {
  identifier: string;
  password: string;
}

export interface LoginResponse {
  access_token: string;
}

export interface RefreshTokenResponse {
  access_token: string;
}

export interface RegisterPayload {
  username: string;
  email: string;
  password: string;
}

export interface RegisterResponse {
  user: {
    id: number;
    username: string;
    created_at: string;
  };
  access_token: string;
}

// Users
export interface UserProfileResponse {
  username: string;
  display_name: string;
  bio: string;
  profile_picture_uuid?: string;
  banner_uuid?: string;
  birth_date: string;
  location: string;
  website: string;
  followers_count: number;
  following_count: number;
  created_at: string;
}

export interface UpdateProfilePayload {
  display_name: string;
  bio: string;
  birth_date: string;
  location: string;
  website: string;
  profile_picture_uuid?: string;
  banner_uuid?: string;
}

// Media
export interface MediaUploadResponse {
  uuids: string[];
}

// Posts
export interface MediaItem {
  uuid: string;
  alt_text: string;
}

export interface CreatePostPayload {
  content: string;
  media: MediaItem[];
  parent_id: number | null;
}

export interface PostAuthor {
  username: string;
  display_name: string;
  profile_picture_uuid?: string;
}

export interface PostEngagementBookmarkCategory {
  id: number;
  name: string;
}

export interface PostEngagement {
  is_liked: boolean;
  is_reposted: boolean;
  is_bookmarked: boolean;
  like_count: number;
  repost_count: number;
  quote_count: number;
  reply_count: number;
  view_count: number;
  bookmark_count: number;
  bookmark_category?: PostEngagementBookmarkCategory | null;
}

export interface Post {
  id: number;
  content: string;
  parent_id: number | null;
  quoted_post_id: number | null;
  created_at: string;
  updated_at: string;
  author: PostAuthor;
  media: MediaItem[];
  engagement: PostEngagement;
}

// Pagination
export interface PaginatedFeedResponse<T = Post> {
  items: T[];
  next_cursor: string | null;
  has_more: boolean;
}

export interface PostWithAncestorsAndDescendants {
  post: Post;
  ancestors?: PaginatedFeedResponse;
  descendants?: PaginatedFeedResponse;
}

// Bookmarks
export interface BookmarkCategory {
  id: number;
  user_id: number;
  name: string;
  color: string;
  created_at: string;
  updated_at: string;
  post_count: number;
}

export interface CreateBookmarkCategoryPayload {
  name: string;
  color?: string;
}

export interface CreateBookmarkCategoryResponse {
  success: boolean;
  category: BookmarkCategory;
}

export interface BookmarkActionResponse {
  success: boolean;
}

// Settings
export interface UserSettings {
  notifications: {
    email: boolean;
    push: boolean;
    mentions: boolean;
  };
  privacy: {
    profileVisibility: 'public' | 'private' | 'friends';
    showOnlineStatus: boolean;
    allowTagging: boolean;
  };
  appearance: {
    theme: 'light' | 'dark' | 'system';
    fontSize: 'small' | 'medium' | 'large';
  };
  language: string;
}

// Recursively optional version of UserSettings. The settings PATCH endpoint
// merges partial nested objects, so this is the correct payload type.
export type DeepPartial<T> = {
  [K in keyof T]?: T[K] extends object ? DeepPartial<T[K]> : T[K];
};