package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
	"unicode/utf8"
)

const commentContentMaxLength = 2048

var (
	// ErrInvalidCommentID denotes an invalid comment ID; that is not uuid.
	ErrInvalidCommentID = InvalidArgumentError("invalid comment ID")
	// ErrCommentNotFound denotes a not found comment.
	ErrCommentNotFound = NotFoundError("comment not found")
)

// Comment model.
type Comment struct {
	ID         string    `json:"id"`
	UserID     string    `json:"-"`
	PostID     string    `json:"-"`
	Content    string    `json:"content"`
	LikesCount int       `json:"likesCount"`
	CreatedAt  time.Time `json:"createdAt"`
	User       *User     `json:"user,omitempty"`
	Mine       bool      `json:"mine"`
	Liked      bool      `json:"liked"`
}

type Comments []Comment

func (cc Comments) EndCursor() *string {
	if len(cc) == 0 {
		return nil
	}

	last := cc[len(cc)-1]
	return ptrString(encodeCursor(last.ID, last.CreatedAt))
}

// CreateComment on a post.
func (s *Service) CreateComment(ctx context.Context, postID string, content string) (Comment, error) {
	var c Comment
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return c, ErrUnauthenticated
	}

	if !reUUID.MatchString(postID) {
		return c, ErrInvalidPostID
	}

	content = smartTrim(content)
	if content == "" || utf8.RuneCountInString(content) > commentContentMaxLength {
		return c, ErrInvalidContent
	}

	tx, err := s.Db.BeginTx(ctx, nil)
	if err != nil {
		return c, fmt.Errorf("could not begin tx: %w", err)
	}

	query := `
			INSERT INTO comments (user_id, post_id, content) VALUES ($1, $2, $3)
			RETURNING id, created_at`
	err = tx.QueryRowContext(ctx, query, uid, postID, content).Scan(&c.ID, &c.CreatedAt)
	if isForeignKeyViolation(err) {
		tx.Rollback()
		return c, ErrPostNotFound
	}

	if err != nil {
		tx.Rollback()
		return c, fmt.Errorf("could not insert comment: %w", err)
	}

	c.UserID = uid
	c.PostID = postID
	c.Content = content
	c.Mine = true

	query = `
			INSERT INTO post_subscriptions (user_id, post_id) VALUES ($1, $2)
			ON CONFLICT (user_id, post_id) DO NOTHING`
	if _, err = tx.ExecContext(ctx, query, uid, postID); err != nil {
		return c, fmt.Errorf("could not insert post subcription after commenting: %w", err)
	}

	query = "UPDATE posts SET comments_count = comments_count + 1 WHERE id = $1"
	if _, err = tx.ExecContext(ctx, query, postID); err != nil {
		tx.Rollback()
		return c, fmt.Errorf("could not update and increment post comments count: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return c, fmt.Errorf("could not commit to create comment: %w", err)
	}

	go s.commentCreated(c)

	return c, nil
}

func (s *Service) commentCreated(c Comment) {
	u, err := s.userByID(context.Background(), c.UserID)
	if err != nil {
		log.Println("error", fmt.Errorf("could not fetch comment user: %w", err))
		return
	}

	c.User = &u
	c.Mine = false

	go s.notifyComment(c)
	go s.notifyCommentMention(c)
	go s.broadcastComment(c)
}

func (s *Service) Comments(ctx context.Context, postID string, last uint64, before *string) (Comments, error) {
	if !reUUID.MatchString(postID) {
		return nil, ErrInvalidPostID
	}

	var beforeCommentID string
	var beforeCreatedAt time.Time

	if before != nil {
		var err error
		beforeCommentID, beforeCreatedAt, err = decodeCursor(*before)
		if err != nil || !reUUID.MatchString(beforeCommentID) {
			return nil, ErrInvalidCursor
		}
	}

	uid, auth := ctx.Value(KeyAuthUserID).(string)
	last = normalizePageSize(last)
	query, args, err := buildQuery(`
		SELECT comments.id
		, comments.content
		, comments.likes_count
		, comments.created_at
		, users.username
		, users.avatar
		{{if .auth}}
		, comments.user_id = @uid AS comment_mine
		, likes.user_id IS NOT NULL AS liked
		{{end}}
		FROM comments
		INNER JOIN users ON comments.user_id = users.id
		{{if .auth}}
		LEFT JOIN comment_likes AS likes
			ON likes.comment_id = comments.id AND likes.user_id = @uid
		{{end}}
		WHERE comments.post_id = @postID
		{{ if and .beforeCommentID .beforeCreatedAt }}
			AND comments.created_at <= @beforeCreatedAt
			AND (
				comments.id != @beforeCommentID
					OR comments.created_at < @beforeCreatedAt
			)
		{{ end }}
		ORDER BY comments.created_at DESC, comments.id ASC
		LIMIT @last`, map[string]interface{}{
		"auth":            auth,
		"uid":             uid,
		"postID":          postID,
		"last":            last,
		"beforeCommentID": beforeCommentID,
		"beforeCreatedAt": beforeCreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build comments sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select comments: %w", err)
	}

	defer rows.Close()

	var cc Comments
	for rows.Next() {
		var c Comment
		var u User
		var avatar sql.NullString
		dest := []interface{}{&c.ID, &c.Content, &c.LikesCount, &c.CreatedAt, &u.Username, &avatar}
		if auth {
			dest = append(dest, &c.Mine, &c.Liked)
		}
		if err = rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("could not scan comment: %w", err)
		}

		u.AvatarURL = s.avatarURL(avatar)
		c.User = &u
		cc = append(cc, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate comment rows: %w", err)
	}

	return cc, nil
}

func (s *Service) ToggleCommentLike(ctx context.Context, commentID string) (ToggleLikeOutput, error) {
	var out ToggleLikeOutput
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return out, ErrUnauthenticated
	}

	if !reUUID.MatchString(commentID) {
		return out, ErrInvalidCommentID
	}

	query := `
			SELECT EXISTS (
				SELECT 1 FROM comment_likes WHERE user_id = $1 AND comment_id = $2
			)`
	if err := s.Db.QueryRowContext(ctx, query, uid, commentID).Scan(&out.Liked); err != nil {
		return out, fmt.Errorf("could not query select comment like existence: %w", err)
	}
	tx, err := s.Db.BeginTx(ctx, nil)
	if err != nil {
		return out, fmt.Errorf("could not begin tx: %w", err)
	}
	if out.Liked {
		query = "DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2"
		if _, err = tx.ExecContext(ctx, query, uid, commentID); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not delete comment like: %w", err)
		}

		query = "UPDATE comments SET likes_count = likes_count - 1 WHERE id = $1 RETURNING likes_count"
		if err = tx.QueryRowContext(ctx, query, commentID).Scan(&out.LikesCount); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not decrement comment likes count: %w", err)
		}
	} else {
		query = "INSERT INTO comment_likes (user_id, comment_id) VALUES ($1, $2)"
		if _, err = tx.ExecContext(ctx, query, uid, commentID); err != nil {
			tx.Rollback()
			if isForeignKeyViolation(err) {
				return out, ErrPostNotFound
			}
			return out, fmt.Errorf("could not insert comment like: %w", err)
		}

		query = "UPDATE comments SET likes_count = likes_count + 1 WHERE id = $1 RETURNING likes_count"
		if err = tx.QueryRowContext(ctx, query, commentID).Scan(&out.LikesCount); err != nil {
			tx.Rollback()
			return out, fmt.Errorf("could not increment comment likes count: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return out, fmt.Errorf("could not commit to toggle comment like: %w", err)
	}

	out.Liked = !out.Liked
	return out, nil
}

// CommentStream to receive comments in realtime.
func (s *Service) CommentStream(ctx context.Context, postID string) <-chan Comment {
	cc := make(chan Comment)
	c := &commentClient{comments: cc, postID: postID, ctx: ctx}
	if uid, ok := ctx.Value(KeyAuthUserID).(string); ok {
		c.userID = &uid
	}
	// Signal the broker that we have a new connection
	s.BrokerRepository.commentBroker.NewClients <- c

	go func() {
		// Listen to connection close and un-register client connection
		<-ctx.Done()
		s.BrokerRepository.commentBroker.ClosingClients <- c
	}()
	return cc
}

func (s *Service) broadcastComment(c Comment) {
	s.BrokerRepository.commentBroker.Notifier <- c
}
