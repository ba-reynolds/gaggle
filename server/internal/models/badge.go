package models

import "time"

// BadgeKind distinguishes auto-computed ("earned") badges from badges
// explicitly granted by an admin ("assigned").
type BadgeKind string

const (
	BadgeKindEarned   BadgeKind = "earned"
	BadgeKindAssigned BadgeKind = "assigned"
)

// Badge is a single badge in the catalog. Earned badges carry a Criteria used
// to compute eligibility on read; assigned badges are stored in user_badges.
type Badge struct {
	ID          int            `json:"id"`
	Key         string         `json:"key"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Kind        BadgeKind      `json:"kind"`
	Criteria    *BadgeCriteria `json:"criteria,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// BadgeCriteria describes the condition that earns an "earned" badge.
type BadgeCriteria struct {
	Metric string `json:"metric"`
	Min    int    `json:"min"`
}

// CreateBadgePayload is the body for creating/updating an admin-assigned badge.
type CreateBadgePayload struct {
	Key         string `json:"key" validate:"required,min=2,max=50"`
	Label       string `json:"label" validate:"required,min=1,max=60"`
	Description string `json:"description" validate:"required,min=1,max=200"`
	Icon        string `json:"icon" validate:"required,min=1,max=50"`
}

// UserBadge is a badge as shown on a profile: catalog fields plus the
// assignment timestamp (nil for earned badges).
type UserBadge struct {
	Badge
	GrantedAt *time.Time `json:"granted_at,omitempty"`
}

// BadgeCriteriaMetrics holds the per-user metrics needed to evaluate every
// earned badge's criteria in a single pass.
type BadgeCriteriaMetrics struct {
	AccountAgeDays int
	PostsCount     int
	FollowersCount int
	LikesReceived  int
}

// Meets reports whether the metric satisfies the given earned-badge criteria.
func (m BadgeCriteriaMetrics) Meets(criteria *BadgeCriteria) bool {
	if criteria == nil {
		return false
	}
	switch criteria.Metric {
	case "account_age_days":
		return m.AccountAgeDays >= criteria.Min
	case "posts_count":
		return m.PostsCount >= criteria.Min
	case "followers_count":
		return m.FollowersCount >= criteria.Min
	case "likes_received":
		return m.LikesReceived >= criteria.Min
	}
	return false
}
