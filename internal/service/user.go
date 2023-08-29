package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

const (
	// MaxAvatarBytes to read.
	MaxAvatarBytes = 5 << 20 // 5MB
	AvatarsBucket  = "avatars"
)

var (
	reEmail    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	reUsername = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,17}$`)
	avatarsDir = path.Join("static", "img", "avatars")
)

var (
	// ErrInvalidUserID denotes an invalid user id; that is not uuid.
	ErrInvalidUserID = InvalidArgumentError("invalid user ID")
	// ErrInvalidEmail denotes an invalid email address.
	ErrInvalidEmail = InvalidArgumentError("invalid email")
	// ErrInvalidUsername denotes an invalid username.
	ErrInvalidUsername = InvalidArgumentError("invalid username")
	// ErrEmailTaken denotes an email already taken.
	ErrEmailTaken = AlreadyExistsError("email taken")
	// ErrUsernameTaken denotes a username already taken.
	ErrUsernameTaken = AlreadyExistsError("username taken")
	// ErrUserNotFound denotes a not found user.
	ErrUserNotFound = NotFoundError("user not found")
	// ErrForbiddenFollow denotes a forbiden follow. Like following yourself.
	ErrForbiddenFollow = PermissionDeniedError("forbidden follow")
	// ErrUnsupportedAvatarFormat denotes an unsupported avatar image format.
	ErrUnsupportedAvatarFormat = InvalidArgumentError("unsupported avatar format")
	// ErrUnsupportedCoverFormat denotes an unsupported avatar image format.
	ErrUnsupportedCoverFormat = InvalidArgumentError("unsupported cover format")
	// ErrUserGone denotes that the user has already been deleted.
	ErrUserGone = GoneError("user gone")
	// ErrInvalidUpdateUserParams denotes invalid params to update a user, that is no params altogether.
	ErrInvalidUpdateUserParams = InvalidArgumentError("invalid update user params")
	// ErrInvalidUserBio denotes an invalid user bio. That is empty or it exceeds the max allowed characters (480).
	ErrInvalidUserBio = InvalidArgumentError("invalid user bio")
	// ErrInvalidUserWaifu denotes an invalid waifu name for an user.
	ErrInvalidUserWaifu = InvalidArgumentError("invalid user waifu")
	// ErrInvalidUserHusbando denotes an invalid husbando name for an user.
	ErrInvalidUserHusbando = InvalidArgumentError("invalid user husbando")
)

type UserProfiles []UserProfile

type User struct {
	ID        string  `json:"id,omitempty"`
	Username  string  `json:"username"`
	AvatarURL *string `json:"avatarURL"`
}

// UserProfile model.
type UserProfile struct {
	User
	Email          string `json:"email,omitempty"`
	FollowersCount int    `json:"followersCount"`
	FolloweesCount int    `json:"followeesCount"`
	Me             bool   `json:"me"`
	Following      bool   `json:"following"`
	Followeed      bool   `json:"followeed"`
}

func (s *Service) CreateUser(ctx context.Context, email, username string) error {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)
	if !reEmail.MatchString(email) {
		return ErrInvalidEmail
	}

	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return ErrInvalidUsername
	}

	query := "INSERT INTO users (email, username) VALUES ($1, $2)"
	_, err := s.Db.ExecContext(ctx, query, email, username)
	if isUniqueViolation(err) {
		if strings.Contains(err.Error(), "email") {
			return ErrEmailTaken
		}

		if strings.Contains(err.Error(), "username") {
			return ErrUsernameTaken
		}
	}
	if err != nil {
		return fmt.Errorf("could not sql insert user: %w", err)
	}
	return nil
}

func (s *Service) userByID(ctx context.Context, id string) (User, error) {
	var u User
	var avatar sql.NullString
	query := "SELECT username, avatar FROM users WHERE id = $1"
	err := s.Db.QueryRowContext(ctx, query, id).Scan(&u.Username, &avatar)
	if err == sql.ErrNoRows {
		return u, ErrUserNotFound
	}

	if err != nil {
		return u, fmt.Errorf("could not query select user: %w", err)
	}
	u.ID = id
	u.AvatarURL = s.avatarURL(avatar)
	return u, nil
}

// ToggleFollowOutput response.
type ToggleFollowOutput struct {
	Following      bool `json:"following"`
	FollowersCount int  `json:"followersCount"`
}

// ToggleFollow between two users.
func (s *Service) ToggleFollow(ctx context.Context, username string) (ToggleFollowOutput, error) {
	var out ToggleFollowOutput
	followerID, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return out, ErrUnauthenticated
	}

	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return out, ErrInvalidUsername
	}

	var followeeID string
	query := "SELECT id FROM users WHERE username = $1"
	err := s.Db.QueryRowContext(ctx, query, username).Scan(&followeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, ErrUserNotFound
		}
		return out, fmt.Errorf("could not query select user id from username: %w", err)
	}
	if followeeID == followerID {
		return out, ErrForbiddenFollow
	}

	query = `
			SELECT EXISTS (
				SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2
			)`
	err = s.Db.QueryRowContext(ctx, query, followerID, followeeID).Scan(&out.Following)
	if err != nil {
		return out, fmt.Errorf("could not query select existence of follow: %w", err)
	}
	// database transaction starts from here
	tx, err := s.Db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	if out.Following {
		query = "DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2"
		_, err = tx.ExecContext(ctx, query, followerID, followeeID)
		if err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not delete follow: %w", err)
		}

		query = "UPDATE users SET followees_count = followees_count - 1 WHERE id = $1"
		if _, err = tx.ExecContext(ctx, query, followerID); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not decrement followees count: %w", err)
		}

		query = `
				UPDATE users SET followers_count = followers_count - 1 WHERE id = $1
				RETURNING followers_count`
		if err = tx.QueryRowContext(ctx, query, followeeID).Scan(&out.FollowersCount); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not decrement followers count: %w", err)
		}
	} else {
		query = "INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)"
		_, err = tx.ExecContext(ctx, query, followerID, followeeID)
		if err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not insert follow: %w", err)
		}

		query = "UPDATE users SET followees_count = followees_count + 1 WHERE id = $1"
		if _, err = tx.ExecContext(ctx, query, followerID); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not increment followees count: %w", err)
		}

		query = `
				UPDATE users SET followers_count = followers_count + 1 WHERE id = $1
				RETURNING followers_count`
		err = tx.QueryRowContext(ctx, query, followeeID).Scan(&out.FollowersCount)
		if err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not increment followers count: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	out.Following = !out.Following

	if out.Following {
		go s.notifyFollow(followerID, followeeID)
	}
	return out, nil
}

// User with the given username.
func (s *Service) User(ctx context.Context, username string) (UserProfile, error) {
	var u UserProfile

	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return u, ErrInvalidUsername
	}

	uid, auth := ctx.Value(KeyAuthUserID).(string)
	query, args, err := buildQuery(`
		SELECT id, email, avatar, followers_count, followees_count
		{{if .auth}}
		, followers.follower_id IS NOT NULL AS following
		, followees.followee_id IS NOT NULL AS followeed
		{{end}}
		FROM users
		{{if .auth}}
		LEFT JOIN follows AS followers
			ON followers.follower_id = @uid AND followers.followee_id = users.id
		LEFT JOIN follows AS followees
			ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{end}}
		WHERE username = @username`, map[string]interface{}{
		"auth":     auth,
		"uid":      uid,
		"username": username,
	})
	if err != nil {
		return u, fmt.Errorf("could not build user sql query: %w", err)
	}

	var avatar sql.NullString
	dest := []interface{}{&u.ID, &u.Email, &avatar, &u.FollowersCount, &u.FolloweesCount}
	if auth {
		dest = append(dest, &u.Following, &u.Followeed)
	}
	err = s.Db.QueryRowContext(ctx, query, args...).Scan(dest...)
	if err == sql.ErrNoRows {
		return u, ErrUserNotFound
	}

	if err != nil {
		return u, fmt.Errorf("could not query select user: %w", err)
	}

	u.Username = username
	u.Me = auth && (uid == u.ID)
	if !u.Me {
		u.ID = ""
		u.Email = ""
	}
	u.AvatarURL = s.avatarURL(avatar)
	return u, nil
}

// Users in ascending order with forward pagination and filtered by username.
func (s *Service) Users(ctx context.Context, search string, first uint64, after string) (UserProfiles, error) {
	search = strings.TrimSpace(search)
	first = normalizePageSize(first)

	uid, auth := ctx.Value(KeyAuthUserID).(string)
	query, args, err := buildQuery(`
		SELECT id, email, username, avatar, followers_count, followees_count
		{{ if .auth }}
		, followers.follower_id IS NOT NULL AS following
		, followees.followee_id IS NOT NULL AS followeed
		{{ end }}
		FROM users
		{{ if .auth }}
		LEFT JOIN follows AS followers
			ON followers.follower_id = @uid AND followers.followee_id = users.id
		LEFT JOIN follows AS followees
			ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{ end }}
		{{ if or .search .after }}WHERE{{ end }}
		{{ if .search }}username ILIKE '%' || @search || '%'{{ end }}
		{{ if and .search .after }}AND{{ end }}
		{{ if .after }}username > @after{{ end }}
		ORDER BY username ASC
		LIMIT @first`, map[string]interface{}{
		"auth":   auth,
		"uid":    uid,
		"search": search,
		"first":  first,
		"after":  after,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build users sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select users: %w", err)
	}

	defer rows.Close()

	var uu UserProfiles
	for rows.Next() {
		var u UserProfile
		var avatar sql.NullString
		dest := []interface{}{
			&u.ID,
			&u.Email,
			&u.Username,
			&avatar,
			&u.FollowersCount,
			&u.FolloweesCount,
		}
		if auth {
			dest = append(dest, &u.Following, &u.Followeed)
		}
		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan user: %w", err)
		}

		u.Me = auth && uid == u.ID
		if !u.Me {
			u.ID = ""
			u.Email = ""
		}
		u.AvatarURL = s.avatarURL(avatar)
		uu = append(uu, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate user rows: %w", err)
	}

	return uu, nil
}

// Followers in ascending order with forward pagination.
func (s *Service) Followers(ctx context.Context, username string, first uint64, after string) (UserProfiles, error) {
	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return nil, ErrInvalidUsername
	}

	first = normalizePageSize(first)
	uid, auth := ctx.Value(KeyAuthUserID).(string)
	query, args, err := buildQuery(`
		SELECT users.id
		, users.email
		, users.username
		, users.avatar
		, users.followers_count
		, users.followees_count
		{{ if .auth }}
		, followers.follower_id IS NOT NULL AS following
		, followees.followee_id IS NOT NULL AS followeed
		{{ end }}
		FROM follows
		INNER JOIN users ON follows.follower_id = users.id
		{{ if .auth }}
		LEFT JOIN follows AS followers
			ON followers.follower_id = @uid AND followers.followee_id = users.id
		LEFT JOIN follows AS followees
			ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{ end }}
		WHERE follows.followee_id = (SELECT id FROM users WHERE username = @username)
		{{ if .after }}AND username > @after{{ end }}
		ORDER BY username ASC
		LIMIT @first`, map[string]interface{}{
		"auth":     auth,
		"uid":      uid,
		"username": username,
		"first":    first,
		"after":    after,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build followers sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select followers: %w", err)
	}

	defer rows.Close()

	var uu UserProfiles
	for rows.Next() {
		var u UserProfile
		var avatar sql.NullString
		dest := []interface{}{
			&u.ID,
			&u.Email,
			&u.Username,
			&avatar,
			&u.FollowersCount,
			&u.FolloweesCount,
		}
		if auth {
			dest = append(dest, &u.Following, &u.Followeed)
		}
		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan follower: %w", err)
		}

		u.Me = auth && uid == u.ID
		if !u.Me {
			u.ID = ""
			u.Email = ""
		}
		u.AvatarURL = s.avatarURL(avatar)
		uu = append(uu, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate follower rows: %w", err)
	}

	return uu, nil
}

// Followees in ascending order with forward pagination.
func (s *Service) Followees(ctx context.Context, username string, first uint64, after string) (UserProfiles, error) {
	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return nil, ErrInvalidUsername
	}

	first = normalizePageSize(first)
	uid, auth := ctx.Value(KeyAuthUserID).(string)
	query, args, err := buildQuery(`
		SELECT users.id
		, users.email
		, users.username
		, users.avatar
		, users.followers_count
		, users.followees_count
		{{ if .auth }}
		, followers.follower_id IS NOT NULL AS following
		, followees.followee_id IS NOT NULL AS followeed
		{{ end }}
		FROM follows
		INNER JOIN users ON follows.followee_id = users.id
		{{ if .auth }}
		LEFT JOIN follows AS followers
			ON followers.follower_id = @uid AND followers.followee_id = users.id
		LEFT JOIN follows AS followees
			ON followees.follower_id = users.id AND followees.followee_id = @uid
		{{ end }}
		WHERE follows.follower_id = (SELECT id FROM users WHERE username = @username)
		{{ if .after }}AND username > @after{{ end }}
		ORDER BY username ASC
		LIMIT @first`, map[string]interface{}{
		"auth":     auth,
		"uid":      uid,
		"username": username,
		"first":    first,
		"after":    after,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build followees sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select followees: %w", err)
	}

	defer rows.Close()

	var uu UserProfiles
	for rows.Next() {
		var u UserProfile
		var avatar sql.NullString
		dest := []interface{}{
			&u.ID,
			&u.Email,
			&u.Username,
			&avatar,
			&u.FollowersCount,
			&u.FolloweesCount,
		}
		if auth {
			dest = append(dest, &u.Following, &u.Followeed)
		}
		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan followee: %w", err)
		}

		u.Me = auth && uid == u.ID
		if !u.Me {
			u.ID = ""
			u.Email = ""
		}
		u.AvatarURL = s.avatarURL(avatar)
		uu = append(uu, u)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate followee rows: %w", err)
	}

	return uu, nil
}

// UpdateAvatar of the authenticated user returning the new avatar URL.
// Please limit the reader before hand using MaxAvatarBytes.
func (s *Service) UpdateAvatar(ctx context.Context, r io.Reader) (string, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return "", ErrUnauthenticated
	}

	r = io.LimitReader(r, MaxAvatarBytes)
	img, format, err := image.Decode(r)
	if err != nil {
		return "", fmt.Errorf("could not read avatar: %w", err)
	}

	if format != "png" && format != "jpg" {
		return "", ErrUnsupportedAvatarFormat
	}
	avatarFileName, err := gonanoid.New()
	if err != nil {
		return "", fmt.Errorf("could not generate avatar filename: %w", err)
	}

	if format == "png" {
		avatarFileName += ".png"
	} else {
		avatarFileName += ".jpg"
	}

	avatarPath := path.Join(avatarsDir, avatarFileName)
	f, err := os.Create(avatarPath)
	if err != nil {
		return "", fmt.Errorf("could not create avatar file: %w", err)
	}
	defer f.Close()

	img = imaging.Fill(img, 400, 400, imaging.Center, imaging.CatmullRom)
	if format == "png" {
		err = png.Encode(f, img)
	} else {
		err = jpeg.Encode(f, img, nil)
	}
	if err != nil {
		return "", fmt.Errorf("could not write avatar to disk: %w", err)
	}

	var oldAvatar sql.NullString
	query := `
		UPDATE users SET avatar = $1 WHERE id = $2
		RETURNING (SELECT avatar FROM users WHERE id = $2) AS old_avatar
	`
	row := s.Db.QueryRowContext(ctx, query, avatarFileName, uid)
	err = row.Scan(&oldAvatar)
	if err != nil {
		defer os.Remove(avatarPath)
		return "", fmt.Errorf("could not update avatar: %w", err)
	}

	if oldAvatar.Valid {
		defer os.Remove(path.Join(avatarsDir, oldAvatar.String))
	}

	return s.AvatarURLPrefix + avatarFileName, nil
}

func (s *Service) avatarURL(avatar sql.NullString) *string {
	if !avatar.Valid {
		return nil
	}
	str := s.AvatarURLPrefix + avatar.String
	return &str
}

func ValidUsername(s string) bool {
	return reUsername.MatchString(s)
}
