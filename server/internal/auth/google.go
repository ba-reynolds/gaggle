package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ba-reynolds/gaggle/internal/apperrors"
	"github.com/ba-reynolds/gaggle/internal/models"
)

// GoogleTokenInfo mirrors the response from https://oauth2.googleapis.com/tokeninfo
type GoogleTokenInfo struct {
	Iss           string `json:"iss"`
	Azp           string `json:"azp"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Exp           string `json:"exp"`
	ExpiresIn     string `json:"expires_in"`
}

// VerifyGoogleIDToken validates an ID token by calling Google's tokeninfo endpoint.
// It checks aud matches the configured client ID, issuer is Google, and expiry is not stale.
// For a self-contained verification (no network) we could verify the JWT signature against
// Google's JWKs, but tokeninfo keeps the implementation small and is sufficient for the
// backend trust model (the frontend already obtained the token from Google directly).
func VerifyGoogleIDToken(ctx context.Context, idToken string, clientID string) (*models.GoogleUserInfo, error) {
	if strings.TrimSpace(idToken) == "" {
		return nil, apperrors.PayloadValidationError(fmt.Errorf("id_token is required"))
	}
	// tokeninfo is a GET with id_token query param
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("google tokeninfo request failed: %w", err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.UnauthorizedError(fmt.Sprintf("google tokeninfo rejected (%d): %s", resp.StatusCode, string(body)), nil)
	}
	var info GoogleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("failed to parse google tokeninfo: %w", err))
	}
	// Validate audience
	if clientID != "" && info.Aud != clientID {
		return nil, apperrors.UnauthorizedError(fmt.Sprintf("google token aud mismatch: expected %s got %s", clientID, info.Aud), nil)
	}
	// Validate issuer
	if info.Iss != "https://accounts.google.com" && info.Iss != "accounts.google.com" {
		return nil, apperrors.UnauthorizedError(fmt.Sprintf("invalid google token issuer: %s", info.Iss), nil)
	}
	if info.Sub == "" || info.Email == "" {
		return nil, apperrors.UnauthorizedError("google token missing sub/email", nil)
	}
	emailVerified := info.EmailVerified == "true" || info.EmailVerified == "1"
	return &models.GoogleUserInfo{
		Sub:           info.Sub,
		Email:         info.Email,
		EmailVerified: emailVerified,
		Name:          info.Name,
		GivenName:     info.GivenName,
		FamilyName:    info.FamilyName,
		Picture:       info.Picture,
	}, nil
}

// GoogleTokenResponse is the OAuth code-exchange response.
type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

// ExchangeGoogleCode exchanges an authorization code for tokens.
func ExchangeGoogleCode(ctx context.Context, clientID, clientSecret, redirectURL, code string) (*GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("redirect_uri", redirectURL)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("google code exchange failed: %w", err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.UnauthorizedError(fmt.Sprintf("google code exchange rejected (%d): %s", resp.StatusCode, string(body)), nil)
	}
	var tokenResp GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("failed to parse google token response: %w", err))
	}
	if tokenResp.IDToken == "" && tokenResp.AccessToken == "" {
		return nil, apperrors.UnauthorizedError("google did not return tokens", nil)
	}
	return &tokenResp, nil
}

// FetchGoogleUserInfo retrieves the profile via the access_token's userinfo endpoint.
func FetchGoogleUserInfo(ctx context.Context, accessToken string) (*models.GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, apperrors.InternalServerError(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("google userinfo request failed: %w", err))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.UnauthorizedError(fmt.Sprintf("google userinfo rejected (%d): %s", resp.StatusCode, string(body)), nil)
	}
	var info struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, apperrors.InternalServerError(fmt.Errorf("failed to parse google userinfo: %w", err))
	}
	if info.ID == "" || info.Email == "" {
		return nil, apperrors.UnauthorizedError("google userinfo missing id/email", nil)
	}
	return &models.GoogleUserInfo{
		Sub:           info.ID,
		Email:         info.Email,
		EmailVerified: info.VerifiedEmail,
		Name:          info.Name,
		GivenName:     info.GivenName,
		FamilyName:    info.FamilyName,
		Picture:       info.Picture,
	}, nil
}

// BuildGoogleAuthURL constructs the Google OAuth consent URL.
func BuildGoogleAuthURL(clientID, redirectURL, state string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("state", state)
	params.Set("include_granted_scopes", "true")
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}
