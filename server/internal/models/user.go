package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            int        `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"-"`
	Password      string     `json:"-"`
	IsAdmin       bool       `json:"-"`
	IsPrivate     bool       `json:"-"`
	SoftDeleted   bool       `json:"-"`
	SoftDeletedAt *time.Time `json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"-"`
}

type UserProfile struct {
	DisplayName        string    `json:"display_name" validate:"required,min=1,max=50"`
	Bio                string    `json:"bio" validate:"required,max=160"`
	ProfilePictureUUID uuid.UUID `json:"profile_picture_uuid"`
	BannerUUID         uuid.UUID `json:"banner_uuid"`
	BirthDate          Date      `json:"birth_date"`
	Location           string    `json:"location" validate:"min=3,max=30"`
	Website            string    `json:"website" validate:"min=3,max=50"`
	FollowersCount     int       `json:"followers_count"`
	FollowingCount     int       `json:"following_count"`
}

type UserWithProfile struct {
	User
	Profile UserProfile `json:"profile"`
	Badges  []UserBadge `json:"badges"`
}

// UserProfileResponse is the flat, frontend-friendly representation of a user's
// profile returned by GET /users/me, GET /users/{username} and PATCH /users/me.
type UserProfileResponse struct {
	UserID             int         `json:"-"`
	Username           string      `json:"username"`
	DisplayName        string      `json:"display_name"`
	Bio                string      `json:"bio"`
	ProfilePictureUUID *uuid.UUID  `json:"profile_picture_uuid,omitempty"`
	BannerUUID         *uuid.UUID  `json:"banner_uuid,omitempty"`
	BirthDate          Date        `json:"birth_date"`
	Location           string      `json:"location"`
	Website            string      `json:"website"`
	FollowersCount     int         `json:"followers_count"`
	FollowingCount     int         `json:"following_count"`
	CreatedAt          time.Time   `json:"created_at"`
	IsAdmin            bool        `json:"is_admin,omitempty"`
	// IsPrivate reports whether the account only shows posts to followers.
	// The profile shell (display name, bio, counts) stays public either way.
	IsPrivate          bool        `json:"is_private"`
	IsFollowing        bool        `json:"is_following"`
	IsBlocked          bool        `json:"is_blocked"`
	IsMuted            bool        `json:"is_muted"`
	Badges             []UserBadge `json:"badges"`
}

// ToProfileResponse converts a UserWithProfile into the API response shape.
func (u *UserWithProfile) ToProfileResponse() UserProfileResponse {
	resp := UserProfileResponse{
		UserID:         u.ID,
		Username:       u.Username,
		DisplayName:    u.Profile.DisplayName,
		Bio:            u.Profile.Bio,
		BirthDate:      u.Profile.BirthDate,
		Location:       u.Profile.Location,
		Website:        u.Profile.Website,
		FollowersCount: u.Profile.FollowersCount,
		FollowingCount: u.Profile.FollowingCount,
		CreatedAt:      u.CreatedAt,
		IsAdmin:        u.IsAdmin,
		IsPrivate:      u.IsPrivate,
		Badges:         u.Badges,
	}
	if u.Badges == nil {
		resp.Badges = []UserBadge{}
	}
	if u.Profile.ProfilePictureUUID != uuid.Nil {
		pp := u.Profile.ProfilePictureUUID
		resp.ProfilePictureUUID = &pp
	}
	if u.Profile.BannerUUID != uuid.Nil {
		b := u.Profile.BannerUUID
		resp.BannerUUID = &b
	}
	return resp
}

type UpdateUserProfileRequest = UserProfile
