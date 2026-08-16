package models

// UserSettings mirrors the frontend settings contract. It is persisted as a
// JSONB row so nested keys can be patched without schema migrations.
type UserSettings struct {
	Notifications NotificationSettings `json:"notifications"`
	Privacy       PrivacySettings      `json:"privacy"`
	Appearance    AppearanceSettings   `json:"appearance"`
	Language      string               `json:"language"`
}

type NotificationSettings struct {
	Email    bool `json:"email"`
	Push     bool `json:"push"`
	Mentions bool `json:"mentions"`
}

type PrivacySettings struct {
	ProfileVisibility string `json:"profileVisibility"`
	ShowOnlineStatus  bool   `json:"showOnlineStatus"`
	AllowTagging      bool   `json:"allowTagging"`
}

type AppearanceSettings struct {
	Theme    string `json:"theme"`
	FontSize string `json:"fontSize"`
}

// UserList is a paginated list of user profile responses (used for feeds like
// "users who liked a post" and "users who reposted a post").
type UserList struct {
	Items      []UserProfileResponse `json:"items"`
	HasMore    bool                  `json:"has_more"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

// Implement PaginatedResponse interface
func (ul *UserList) GetHasMore() bool {
	return ul.HasMore
}

func (ul *UserList) GetNextCursor() string {
	return ul.NextCursor
}
