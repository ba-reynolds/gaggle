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
  language?: string;
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
export interface Badge {
  id: number;
  key: string;
  label: string;
  description: string;
  icon: string;
  kind: 'earned' | 'assigned';
  criteria?: { metric: string; min: number };
  created_at: string;
}

export interface UserBadge extends Badge {
  granted_at?: string;
}

// Admin metrics dashboard
export interface HostMetrics {
  cpu_percent: number;
  mem_total: number;
  mem_used: number;
  mem_percent: number;
  load1: number;
  load5: number;
  load15: number;
  uptime_seconds: number;
  disk_total: number;
  disk_used: number;
  disk_percent: number;
}

export interface AppStats {
  users: number;
  posts: number;
  likes: number;
  messages: number;
  views_total: number;
  signups_24h: number;
}

export interface ActiveUsers {
  dau: number;
  wau: number;
}

export interface DayViewCount {
  day: string;
  views: number;
}

export interface ViewStats {
  requests_per_minute: number;
  by_day: DayViewCount[];
}

export interface AdminMetrics {
  host: HostMetrics;
  app: AppStats;
  active: ActiveUsers;
  views: ViewStats;
}

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
  is_admin?: boolean;
  is_private?: boolean;
  is_following?: boolean;
  is_blocked?: boolean;
  is_muted?: boolean;
  badges: UserBadge[];
}

export interface CreateBadgePayload {
  key: string;
  label: string;
  description: string;
  icon: string;
}

// Lists
export interface List {
  id: number;
  owner_id: number;
  owner_username: string;
  name: string;
  description: string;
  member_count: number;
  created_at: string;
}

export interface CreateListPayload {
  name: string;
  description: string;
}

// Direct messages
export interface MessageSender {
  username: string;
  display_name: string;
  profile_picture_uuid?: string;
}

export interface Message {
  id: number;
  conversation_id: number;
  sender_id: number;
  sender: MessageSender;
  body: string;
  read_at?: string;
  created_at: string;
  // True for locally-inserted messages shown before the server confirms them.
  pending?: boolean;
}

export interface ConversationOtherParticipant {
  username: string;
  display_name: string;
  profile_picture_uuid?: string;
}

export interface Conversation {
  id: number;
  created_at: string;
  last_message_at: string;
  other_participant: ConversationOtherParticipant;
  last_message?: Message;
  unread_count: number;
}

export interface SendMessagePayload {
  body: string;
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

export interface NewsLink {
  url: string;
  title: string;
  image_url: string;
  site_name: string;
}

export interface CreatePostPayload {
  content: string;
  media: MediaItem[];
  parent_id: number | null;
  poll?: CreatePollPayload;
  news?: NewsLink;
  visibility: 'public' | 'followers' | 'mentions';
}

export interface CreatePollPayload {
  question: string;
  options: string[];
  ends_at?: string;
}

export interface PostAuthor {
  username: string;
  display_name: string;
  profile_picture_uuid?: string;
}

export interface PostParent {
  id: number;
  deleted: boolean;
  author?: PostAuthor;
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
  edited_at?: string;
  is_pinned: boolean;
  visibility: 'public' | 'followers' | 'mentions';
  poll?: Poll;
  news?: NewsLink;
  parent?: PostParent;
}

export interface PollOption {
  id: number;
  label: string;
  position: number;
  vote_count: number;
}

export interface Poll {
  id: number;
  question: string;
  ends_at?: string;
  options: PollOption[];
  total_votes: number;
  selected_option_id?: number;
  closed: boolean;
}

export interface PostEdit {
  id: number;
  content_before: string;
  edited_at: string;
}

// Pagination
export interface PaginatedFeedResponse<T = Post> {
  items: T[];
  next_cursor: string | null;
  has_more: boolean;
}

export interface Notification {
  id: number;
  type: 'like' | 'repost' | 'quote' | 'reply' | 'follow' | 'mention';
  actor: PostAuthor;
  post_id?: number;
  read_at?: string;
  created_at: string;
}

export interface Trend {
  name: string;
  count: number;
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
