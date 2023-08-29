package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"github.com/pascaldekloe/jwt"
	"strings"
	"time"
)

type ctxkey string

const KeyAuthUserID = ctxkey("auth_user_id")

const (
	OTP_TTL      = time.Second * 30
	authTokenTTL = time.Hour * 24 * 14
)

var (
	// ErrInvalidRedirectURI denotes an invalid redirect URI.
	ErrInvalidRedirectURI = InvalidArgumentError("invalid redirect URI")
	// ErrUntrustedRedirectURI denotes an untrusted redirect URI.
	// That is an URI that is not in the same host as the nakama.
	ErrUntrustedRedirectURI = PermissionDeniedError("untrusted redirect URI")
	// ErrInvalidToken denotes an invalid token.
	ErrInvalidToken = InvalidArgumentError("invalid token")
	// ErrExpiredToken denotes that the token already expired.
	ErrExpiredToken = UnauthenticatedError("expired token")
	// ErrInvalidVerificationCode denotes an invalid verification code.
	ErrInvalidVerificationCode = InvalidArgumentError("invalid verification code")
	// ErrVerificationCodeNotFound denotes a not found verification code.
	ErrVerificationCodeNotFound = NotFoundError("verification code not found")
)

type TokenOutput struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type OTPOutput struct {
	OTP       string    `json:"otp"`
	ExpiresAt time.Time `json:"expiresAt"`
	Hash      []byte    `json:"-"`
}

type AuthOutput struct {
	User      User      `json:"user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Service) Login(ctx context.Context, email string) (AuthOutput, error) {
	var out AuthOutput

	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	if !reEmail.MatchString(email) {
		return out, ErrInvalidEmail
	}

	var avatar sql.NullString
	query := "SELECT id, username, avatar FROM users WHERE email = $1"
	err := s.Db.QueryRowContext(ctx, query, email).Scan(&out.User.ID, &out.User.Username, &avatar)

	if err == sql.ErrNoRows {
		return out, ErrUserNotFound
	}

	if err != nil {
		return out, fmt.Errorf("could not query select user: %w", err)
	}

	out.User.AvatarURL = s.avatarURL(avatar)

	claims := NewClaims(out.User.ID)

	jwtBytes, err := claims.HMACSign(jwt.HS256, []byte(s.JWTSecret))
	if err != nil {
		return out, fmt.Errorf("could not create token: %w", err)
	}

	out.Token = string(jwtBytes)

	out.ExpiresAt = time.Now().Add(authTokenTTL)

	return out, nil
}

// Token to authenticate requests.
func (s *Service) Token(ctx context.Context) (TokenOutput, error) {
	var out TokenOutput
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return out, ErrUnauthenticated
	}

	claims := NewClaims(uid)

	jwtBytes, err := claims.HMACSign(jwt.HS256, []byte(s.JWTSecret))
	if err != nil {
		return out, fmt.Errorf("could not create token: %w", err)
	}

	out.Token = string(jwtBytes)
	out.ExpiresAt = time.Now().Add(authTokenTTL)

	return out, nil
}

// AuthUser is the current authenticated user.
func (s *Service) AuthUser(ctx context.Context) (User, error) {
	var u User
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return u, ErrUnauthenticated
	}

	return s.userByID(ctx, uid)
}

func (s *Service) AuthUserIDFromToken(token string) (string, error) {
	claims, err := jwt.HMACCheck([]byte(token), []byte(s.JWTSecret))
	if err != nil {
		return "", ErrInvalidToken
	}

	if !claims.Valid(time.Now()) {
		return "", ErrExpiredToken
	}

	return claims.Subject, nil
}

func (s *Service) OTP(ctx context.Context) (OTPOutput, error) {
	var out OTPOutput
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return out, ErrUnauthenticated
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return out, err
	}
	out.OTP = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(out.OTP))
	out.Hash = hash[:]
	out.ExpiresAt = time.Now().Add(OTP_TTL)

	query := `
INSERT INTO otps (hash, user_id, expiry)
VALUES ($1, $2, $3)`
	args := []interface{}{out.Hash, uid, out.ExpiresAt}
	_, err = s.Db.ExecContext(ctx, query, args...)
	return out, err
}

func (s *Service) AuthUserIDFromOTP(otp string, ctx context.Context) (string, error) {
	otpHash := sha256.Sum256([]byte(otp))
	query := `
SELECT users.id
FROM users
INNER JOIN otps
ON users.id = otps.user_id
WHERE otps.hash = $1
AND otps.expiry > $2`
	args := []interface{}{otpHash[:], time.Now()}
	var uid string
	err := s.Db.QueryRowContext(ctx, query, args...).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	return uid, nil
}

func (s *Service) DeleteAllOTPForUser(userID string, ctx context.Context) error {
	query := `
DELETE FROM otps WHERE user_id = $1`
	_, err := s.Db.ExecContext(ctx, query, userID)
	return err
}
