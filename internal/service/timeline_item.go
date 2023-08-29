package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
	"unicode/utf8"
)

const (
	MaxMediaItemBytes = 5 << 20  // 5MB
	MaxMediaBytes     = 15 << 20 // 15MB
)

var (
	// ErrInvalidTimelineItemID denotes an invalid timeline item id; that is not uuid.
	ErrInvalidTimelineItemID = InvalidArgumentError("invalid timeline item ID")
	// ErrUnsupportedMediaItemFormat denotes an unsupported media item format.
	ErrUnsupportedMediaItemFormat = InvalidArgumentError("unsupported media item format")
	ErrMediaItemTooLarge          = InvalidArgumentError("media item too large")
	ErrMediaTooLarge              = InvalidArgumentError("media too large")
)

type TimelineItem struct {
	ID     string `json:"timelineItemID"`
	UserID string `json:"-"`
	PostID string `json:"-"`
	*Post  `json:"post"`
}

func (tt Timeline) EndCursor() *string {
	if len(tt) == 0 {
		return nil
	}

	last := tt[len(tt)-1]
	if last.Post == nil {
		return nil
	}

	return ptrString(encodeCursor(last.Post.ID, last.Post.CreatedAt))
}

// Timeline of the authenticated user in descending order and with backward pagination.
func (s *Service) Timeline(ctx context.Context, last uint64, before *string) (Timeline, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return nil, ErrUnauthenticated
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
	last = normalizePageSize(last)

	query, args, err := buildQuery(`
		SELECT timeline.id
		, posts.id
		, posts.content
		, posts.spoiler_of
		, posts.nsfw
		, posts.likes_count
		, posts.comments_count
		, posts.created_at
		, posts.user_id = @uid AS post_mine
		, likes.user_id IS NOT NULL AS liked
		, subscriptions.user_id IS NOT NULL AS subscribed
		, users.username
		, users.avatar
		FROM timeline
		INNER JOIN posts ON timeline.post_id = posts.id
		INNER JOIN users ON posts.user_id = users.id
		LEFT JOIN post_likes AS likes
			ON likes.user_id = @uid AND likes.post_id = posts.id
		LEFT JOIN post_subscriptions AS subscriptions
			ON subscriptions.user_id = @uid AND subscriptions.post_id = posts.id
		WHERE timeline.user_id = @uid
		{{ if and .beforePostID .beforeCreatedAt }}
			AND posts.created_at <= @beforeCreatedAt
			AND (
				posts.id != @beforePostID
					OR posts.created_at < @beforeCreatedAt
			)
		{{ end }}
		ORDER BY posts.created_at DESC, posts.id ASC
		LIMIT @last`, map[string]interface{}{
		"uid":             uid,
		"last":            last,
		"beforePostID":    beforePostID,
		"beforeCreatedAt": beforeCreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build timeline sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select timeline: %w", err)
	}

	defer rows.Close()

	var tt Timeline
	for rows.Next() {
		var ti TimelineItem
		var p Post
		var u User
		var avatar sql.NullString
		if err = rows.Scan(
			&ti.ID,
			&p.ID,
			&p.Content,
			&p.SpoilerOf,
			&p.NSFW,
			&p.LikesCount,
			&p.CommentsCount,
			&p.CreatedAt,
			&p.Mine,
			&p.Liked,
			&p.Subscribed,
			&u.Username,
			&avatar,
		); err != nil {
			return nil, fmt.Errorf("could not scan timeline item: %w", err)
		}
		u.AvatarURL = s.avatarURL(avatar)
		p.User = &u
		ti.Post = &p
		tt = append(tt, ti)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate timeline rows: %w", err)
	}

	return tt, nil
}

// CreateTimelineItem publishes a post to the user timeline and fan-outs it to his followers.
func (s *Service) CreateTimelineItem(ctx context.Context, content string, spoilerOf *string, nsfw bool) (TimelineItem, error) {
	var ti TimelineItem
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return ti, ErrUnauthenticated
	}

	content = smartTrim(content)
	if content == "" || utf8.RuneCountInString(content) > postContentMaxLength {
		return ti, ErrInvalidContent
	}

	if spoilerOf != nil {
		*spoilerOf = smartTrim(*spoilerOf)
		if *spoilerOf == "" || utf8.RuneCountInString(*spoilerOf) > postSpoilerMaxLength {
			return ti, ErrInvalidSpoiler
		}
	}

	tx, err := s.Db.BeginTx(ctx, nil)
	if err != nil {
		return ti, fmt.Errorf("could not begin tx: %w", err)
	}

	var p Post
	query := `
			INSERT INTO posts (user_id, content, spoiler_of, nsfw) VALUES ($1, $2, $3, $4)
			RETURNING id, created_at`
	err = tx.QueryRowContext(ctx, query, uid, content, spoilerOf, nsfw).Scan(&p.ID, &p.CreatedAt)

	if err != nil {
		tx.Rollback()
		if isForeignKeyViolation(err) {
			return ti, ErrUserGone
		}
		return ti, fmt.Errorf("could not insert post: %w", err)
	}

	p.UserID = uid
	p.Content = content
	p.SpoilerOf = spoilerOf
	p.NSFW = nsfw
	p.Mine = true

	query = "INSERT INTO post_subscriptions (user_id, post_id) VALUES ($1, $2)"
	if _, err = tx.ExecContext(ctx, query, uid, p.ID); err != nil {
		return ti, fmt.Errorf("could not insert post subscription: %w", err)
	}

	p.Subscribed = true

	query = "INSERT INTO timeline (user_id, post_id) VALUES ($1, $2) RETURNING id"
	err = tx.QueryRowContext(ctx, query, uid, p.ID).Scan(&ti.ID)
	if err != nil {
		tx.Rollback()
		return ti, fmt.Errorf("could not insert timeline item: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return ti, fmt.Errorf("could not insert timeline item: %w", err)
	}

	ti.UserID = uid
	ti.PostID = p.ID
	ti.Post = &p

	go s.postCreated(p)

	return ti, nil
}

func (s *Service) postCreated(p Post) {
	u, err := s.userByID(context.Background(), p.UserID)
	if err != nil {
		log.Println(err)
		return
	}

	p.User = &u
	p.Mine = false
	p.Subscribed = false

	go s.fanoutPost(p)
	go s.notifyPostMention(p)
}

type Timeline []TimelineItem

func (s *Service) fanoutPost(p Post) {
	query := `
		INSERT INTO timeline (user_id, post_id)
		SELECT follower_id, $1 FROM follows WHERE followee_id = $2
		RETURNING id, user_id`
	rows, err := s.Db.Query(query, p.ID, p.UserID)
	if err != nil {
		log.Println(fmt.Errorf("could not insert timeline: %w", err))
		return
	}

	defer rows.Close()

	for rows.Next() {
		var ti TimelineItem
		if err = rows.Scan(&ti.ID, &ti.UserID); err != nil {
			log.Println(fmt.Errorf("could not scan timeline item: %w", err))
			return
		}

		ti.PostID = p.ID
		ti.Post = &p

		go s.broadcastTimelineItem(ti)
	}

	if err = rows.Err(); err != nil {
		log.Println(fmt.Errorf("could not iterate timeline rows: %w", err))
		return
	}
}

func (s *Service) TimelineItemStream(ctx context.Context) (<-chan TimelineItem, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return nil, ErrUnauthenticated
	}

	tt := make(chan TimelineItem)
	c := &timelineItemClient{timelines: tt, userID: uid, ctx: ctx}
	// Signal the broker that we have a new connection
	s.BrokerRepository.timelineItemBroker.NewClients <- c

	go func() {
		// Listen to connection close and un-register client connection
		<-ctx.Done()
		s.BrokerRepository.timelineItemBroker.ClosingClients <- c
	}()
	return tt, nil
}

func (s *Service) broadcastTimelineItem(ti TimelineItem) {
	s.BrokerRepository.timelineItemBroker.Notifier <- ti
}
