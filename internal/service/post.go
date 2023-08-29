package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	postContentMaxLength = 2048
	postSpoilerMaxLength = 64
)

var (
	// ErrInvalidPostID denotes an invalid post ID; that is not uuid.
	ErrInvalidPostID = InvalidArgumentError("invalid post ID")
	// ErrInvalidContent denotes an invalid content.
	ErrInvalidContent = InvalidArgumentError("invalid content")
	// ErrInvalidSpoiler denotes an invalid spoiler title.
	ErrInvalidSpoiler = InvalidArgumentError("invalid spoiler")
	// ErrPostNotFound denotes a not found post.
	ErrPostNotFound = NotFoundError("post not found")
	// ErrInvalidUpdatePostParams denotes invalid params to update a post, that is no params altogether.
	ErrInvalidUpdatePostParams = InvalidArgumentError("invalid update post params")
	// ErrInvalidCursor denotes an invalid cursor, that is not base64 encoded and has a key and timestamp separated by comma.
	ErrInvalidCursor = InvalidArgumentError("invalid cursor")
	// ErrInvalidReaction denotes an invalid reaction, that may by an invalid reaction type, or invalid reaction by itslef,
	// not a valid emoji, or invalid reaction image URL.
	ErrInvalidReaction  = InvalidArgumentError("invalid reaction")
	ErrUpdatePostDenied = PermissionDeniedError("update post denied")
)

type Post struct {
	ID            string    `json:"id"`
	UserID        string    `json:"-"`
	Content       string    `json:"content"`
	SpoilerOf     *string   `json:"spoilerOf"`
	NSFW          bool      `json:"nsfw"`
	LikesCount    int       `json:"likesCount"`
	CommentsCount int       `json:"commentsCount"`
	CreatedAt     time.Time `json:"createdAt"`
	User          *User     `json:"user,omitempty"`
	Mine          bool      `json:"mine"`
	Liked         bool      `json:"liked"`
	Subscribed    bool      `json:"subscribed"`
}

type Posts []Post

func (pp Posts) EndCursor() *string {
	if len(pp) == 0 {
		return nil
	}

	last := pp[len(pp)-1]
	return ptrString(encodeCursor(last.ID, last.CreatedAt))
}

func (s *Service) Posts(ctx context.Context, username string, last uint64, before *string) (Posts, error) {
	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return nil, ErrInvalidUsername
	}
	var beforePostID string
	var beforeCreatedAt time.Time
	if before != nil {
		var err error
		beforePostID, beforeCreatedAt, err = decodeCursor(*before)
		if err != nil || !reUUID.MatchString(beforePostID) {
			return nil, ErrInvalidCursor
		}
	}
	uid, auth := ctx.Value(KeyAuthUserID).(string)
	last = normalizePageSize(last)
	query, args, err := buildQuery(`
		SELECT posts.id
		, posts.content
		, posts.spoiler_of
		, posts.nsfw
		, posts.likes_count
		, posts.comments_count
		, posts.created_at
		{{ if .auth }}
		, posts.user_id = @uid AS post_mine
		, likes.user_id IS NOT NULL AS liked
		, subscriptions.user_id IS NOT NULL AS subscribed
		{{ end }}
		FROM posts
		{{if .auth}}
		LEFT JOIN post_likes AS likes
			ON likes.user_id = @uid AND likes.post_id = posts.id
		LEFT JOIN post_subscriptions AS subscriptions
			ON subscriptions.user_id = @uid AND subscriptions.post_id = posts.id
		{{end}}
		WHERE posts.user_id = (SELECT id FROM users WHERE username = @username)
		{{ if and .beforePostID .beforeCreatedAt }}
		AND posts.created_at <= @beforeCreatedAt
		AND (posts.id != @beforePostID OR posts.created_at < @beforeCreatedAt)
		{{end}}
		ORDER BY posts.created_at DESC, posts.id ASC
		LIMIT @last`, map[string]interface{}{
		"auth":            auth,
		"uid":             uid,
		"username":        username,
		"last":            last,
		"beforePostID":    beforePostID,
		"beforeCreatedAt": beforeCreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build posts sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select posts: %w", err)
	}

	defer rows.Close()

	var pp Posts
	for rows.Next() {
		var p Post
		dest := []interface{}{
			&p.ID,
			&p.Content,
			&p.SpoilerOf,
			&p.NSFW,
			&p.LikesCount,
			&p.CommentsCount,
			&p.CreatedAt,
		}
		if auth {
			dest = append(dest, &p.Mine, &p.Liked, &p.Subscribed)
		}

		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan post: %w", err)
		}

		pp = append(pp, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate posts rows: %w", err)
	}

	return pp, nil
}

func (s *Service) Post(ctx context.Context, postID string) (Post, error) {
	var p Post
	if !reUUID.MatchString(postID) {
		return p, ErrInvalidPostID
	}

	uid, auth := ctx.Value(KeyAuthUserID).(string)
	query, args, err := buildQuery(`
		SELECT posts.id
		, posts.content
		, posts.spoiler_of
		, posts.nsfw
		, posts.likes_count
		, posts.comments_count
		, posts.created_at
		, users.username
		, users.avatar
		{{ if .auth }}
		, posts.user_id = @uid AS post_mine
		, likes.user_id IS NOT NULL AS liked
		, subscriptions.user_id IS NOT NULL AS subscribed
		{{ end }}
		FROM posts
		INNER JOIN users 
			ON posts.user_id = users.id
		{{if .auth}}
		LEFT JOIN post_likes AS likes
			ON likes.user_id = @uid AND likes.post_id = posts.id
		LEFT JOIN post_subscriptions AS subscriptions
			ON subscriptions.user_id = @uid AND subscriptions.post_id = posts.id
		{{end}}
		WHERE posts.id = @post_id`, map[string]interface{}{
		"auth":    auth,
		"uid":     uid,
		"post_id": postID,
	})
	if err != nil {
		return p, fmt.Errorf("could not build posts sql query: %w", err)
	}

	var u User
	var avatar sql.NullString
	dest := []interface{}{
		&p.ID,
		&p.Content,
		&p.SpoilerOf,
		&p.NSFW,
		&p.LikesCount,
		&p.CommentsCount,
		&p.CreatedAt,
		&u.Username,
		&avatar,
	}
	if auth {
		dest = append(dest, &p.Mine, &p.Liked, &p.Subscribed)
	}
	err = s.Db.QueryRowContext(ctx, query, args...).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPostNotFound
	}

	if err != nil {
		return p, fmt.Errorf("could not query select post: %w", err)
	}

	u.AvatarURL = s.avatarURL(avatar)
	p.User = &u

	return p, nil
}

type ToggleLikeOutput struct {
	Liked      bool `json:"liked"`
	LikesCount int  `json:"likesCount"`
}

func (s *Service) TogglePostLike(ctx context.Context, postID string) (ToggleLikeOutput, error) {
	var out ToggleLikeOutput
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return out, ErrUnauthenticated
	}

	if !reUUID.MatchString(postID) {
		return out, ErrInvalidPostID
	}

	query := `
		SELECT EXISTS (
			SELECT 1 FROM post_likes WHERE user_id = $1 AND post_id = $2
		)`
	if err := s.Db.QueryRowContext(ctx, query, uid, postID).Scan(&out.Liked); err != nil {
		return out, fmt.Errorf("could not query select post like existence")
	}

	tx, err := s.Db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("cound not begin tx: %w", err)
	}

	if out.Liked {
		query = "DELETE FROM post_likes WHERE user_id = $1 AND post_id = $2"
		if _, err = tx.ExecContext(ctx, query, uid, postID); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not delete post like: %w", err)
		}

		query = "UPDATE posts SET likes_count = likes_count - 1 WHERE id = $1 RETURNING likes_count"
		if err = tx.QueryRowContext(ctx, query, postID).Scan(&out.LikesCount); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not decrement post likes count: %w", err)
		}
	} else {
		query = "INSERT INTO post_likes (user_id, post_id) VALUES ($1, $2)"
		if _, err = tx.ExecContext(ctx, query, uid, postID); err != nil {
			tx.Rollback()
			if isForeignKeyViolation(err) {
				return out, ErrPostNotFound
			}
			return out, fmt.Errorf("could not insert post like: %w", err)
		}

		query = "UPDATE posts SET likes_count = likes_count + 1 WHERE id = $1 RETURNING likes_count"
		if err = tx.QueryRowContext(ctx, query, postID).Scan(&out.LikesCount); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not increment post likes count: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return out, fmt.Errorf("could not commit to toggle post like: %w", err)
	}

	out.Liked = !out.Liked

	return out, nil
}

// ToggleSubscriptionOutput response.
type ToggleSubscriptionOutput struct {
	Subscribed bool `json:"subscribed"`
}

// TogglePostSubscription so you can stop receiving notifications from a thread.
func (s *Service) TogglePostSubscription(ctx context.Context, postID string) (ToggleSubscriptionOutput, error) {
	var out ToggleSubscriptionOutput
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return out, ErrUnauthenticated
	}

	if !reUUID.MatchString(postID) {
		return out, ErrInvalidPostID
	}

	query := `SELECT EXISTS (
			SELECT 1 FROM post_subscriptions WHERE user_id = $1 AND post_id = $2
		)`
	err := s.Db.QueryRowContext(ctx, query, uid, postID).Scan(&out.Subscribed)
	if err != nil {
		return out, fmt.Errorf("could not query select post subscription existence: %w", err)
	}

	if out.Subscribed {
		query = "DELETE FROM post_subscriptions WHERE user_id = $1 AND post_id = $2"
		if _, err = s.Db.ExecContext(ctx, query, uid, postID); err != nil {
			return out, fmt.Errorf("could not delete post subscription: %w", err)
		}
	} else {
		query = "INSERT INTO post_subscriptions (user_id, post_id) VALUES ($1, $2)"
		_, err = s.Db.ExecContext(ctx, query, uid, postID)
		if isForeignKeyViolation(err) {
			return out, ErrPostNotFound
		}
		if err != nil {
			return out, fmt.Errorf("could not insert post subscription: %w", err)
		}
	}

	out.Subscribed = !out.Subscribed

	return out, nil
}
