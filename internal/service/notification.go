package service

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"log"
	"time"
)

// ErrInvalidNotificationID denotes an invalid notification id; that is not uuid.
var ErrInvalidNotificationID = InvalidArgumentError("invalid notification ID")

// Notification model.
type Notification struct {
	ID       string    `json:"id"`
	UserID   string    `json:"-"`
	Actors   []string  `json:"actors"`
	Type     string    `json:"type"`
	PostID   *string   `json:"postID,omitempty"`
	Read     bool      `json:"read"`
	IssuedAt time.Time `json:"issuedAt"`
}

type Notifications []Notification

func (pp Notifications) EndCursor() *string {
	if len(pp) == 0 {
		return nil
	}

	last := pp[len(pp)-1]
	return ptrString(encodeCursor(last.ID, last.IssuedAt))
}

// Notifications from the authenticated user in descending order with backward pagination.
func (s *Service) Notifications(ctx context.Context, last uint64, before *string) (Notifications, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return nil, ErrUnauthenticated
	}

	var beforeNotificationID string
	var beforeIssuedAt time.Time

	if before != nil {
		var err error
		beforeNotificationID, beforeIssuedAt, err = decodeCursor(*before)
		if err != nil || !reUUID.MatchString(beforeNotificationID) {
			return nil, ErrInvalidCursor
		}
	}

	last = normalizePageSize(last)
	query, args, err := buildQuery(`
		SELECT id
		, actors
		, type
		, post_id
		, read_at
		, issued_at
		FROM notifications
		WHERE user_id = @uid
		{{ if and .beforeNotificationID .beforeIssuedAt }}
			AND issued_at <= @beforeIssuedAt
			AND (
				id != @beforeNotificationID
					OR issued_at < @beforeIssuedAt
			)
		{{ end }}
		ORDER BY issued_at DESC, id ASC
		LIMIT @last`, map[string]interface{}{
		"uid":                  uid,
		"last":                 last,
		"beforeNotificationID": beforeNotificationID,
		"beforeIssuedAt":       beforeIssuedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build notifications sql query: %w", err)
	}

	rows, err := s.Db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not query select notifications: %w", err)
	}

	defer rows.Close()

	var nn Notifications
	for rows.Next() {
		var n Notification
		var readAt *time.Time
		if err = rows.Scan(&n.ID, pq.Array(&n.Actors), &n.Type, &n.PostID, &readAt, &n.IssuedAt); err != nil {
			return nil, fmt.Errorf("could not scan notification: %w", err)
		}
		n.Read = readAt != nil
		nn = append(nn, n)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate over notification rows: %w", err)
	}

	return nn, nil
}

// MarkNotificationAsRead sets a notification from the authenticated user as read.
func (s *Service) MarkNotificationAsRead(ctx context.Context, notificationID string) error {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return ErrUnauthenticated
	}

	if !reUUID.MatchString(notificationID) {
		return ErrInvalidNotificationID
	}

	if _, err := s.Db.Exec(`
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, notificationID, uid); err != nil {
		return fmt.Errorf("could not update and mark notification as read: %w", err)
	}

	return nil
}

// MarkNotificationsAsRead sets all notification from the authenticated user as read.
func (s *Service) MarkNotificationsAsRead(ctx context.Context) error {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return ErrUnauthenticated
	}

	if _, err := s.Db.Exec(`
		UPDATE notifications SET read_at = now()
		WHERE user_id = $1 AND read_at IS NULL
	`, uid); err != nil {
		return fmt.Errorf("could not update and mark notifications as read: %w", err)
	}

	return nil
}

// HasUnreadNotifications checks if the authenticated user has any unread notification.
func (s *Service) HasUnreadNotifications(ctx context.Context) (bool, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return false, ErrUnauthenticated
	}

	var unread bool
	if err := s.Db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM notifications WHERE user_id = $1 AND read_at IS NULL)`, uid).Scan(&unread); err != nil {
		return false, fmt.Errorf("could not query select unread notifications existence: %w", err)
	}

	return unread, nil
}

func (s *Service) notifyFollow(followerID, followeeID string) {
	ctx := context.Background()
	var n Notification
	var notified bool

	var actor string
	query := "SELECT username FROM users WHERE id = $1"
	err := s.Db.QueryRowContext(ctx, query, followerID).Scan(&actor)
	if err != nil {
		log.Println(fmt.Errorf("could not query select follow notification actor: %w", err))
		return
	}

	tx, err := s.Db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("could not begin tx: %w", err)
		return
	}
	query = `SELECT EXISTS (
			SELECT 1 FROM notifications
			WHERE user_id = $1
				AND $2 = ANY(actors)
				AND type = 'follow'
		)`
	err = tx.QueryRowContext(ctx, query, followeeID, actor).Scan(&notified)
	if err != nil {
		log.Println(fmt.Errorf("could not query select follow notification existence: %w", err))
		return
	}

	if notified {
		return
	}

	var nid string
	query = "SELECT id FROM notifications WHERE user_id = $1 AND type = 'follow' AND read_at IS NULL"
	err = tx.QueryRowContext(ctx, query, followeeID).Scan(&nid)
	if err != nil && err != sql.ErrNoRows {
		log.Println(fmt.Errorf("could not query select unread follow notification: %w", err))
		return
	}

	if err == sql.ErrNoRows {
		actors := []string{actor}
		query = `
				INSERT INTO notifications (user_id, actors, type) VALUES ($1, $2, 'follow')
				RETURNING id, issued_at`
		row := tx.QueryRowContext(ctx, query, followeeID, pq.Array(actors))
		err = row.Scan(&n.ID, &n.IssuedAt)
		if err != nil {
			tx.Rollback()
			log.Println(fmt.Errorf("could not insert follow notification: %w", err))
			return
		}

		n.Actors = actors
	} else {
		query = `
				UPDATE notifications SET
					actors = array_prepend($1, notifications.actors),
					issued_at = now()
				WHERE id = $2
				RETURNING actors, issued_at`
		row := tx.QueryRowContext(ctx, query, actor, nid)
		err = row.Scan(pq.Array(&n.Actors), &n.IssuedAt)
		if err != nil {
			tx.Rollback()
			log.Println(fmt.Errorf("could not update follow notification: %w", err))
			return
		}

		n.ID = nid
	}

	n.UserID = followeeID
	n.Type = "follow"

	if err = tx.Commit(); err != nil {
		log.Printf("could not commit to notify follow: %w", err)
		return
	}

	if !notified {
		go s.broadcastNotification(n)
	}
}

func (s *Service) notifyComment(c Comment) {
	actor := c.User.Username
	rows, err := s.Db.Query(`
		INSERT INTO notifications (user_id, actors, type, post_id)
		SELECT user_id, $1, 'comment', $2 FROM post_subscriptions
		WHERE post_subscriptions.user_id != $3
			AND post_subscriptions.post_id = $2
		ON CONFLICT (user_id, type, post_id, read_at) DO UPDATE SET
			actors = array_prepend($4, array_remove(notifications.actors, $4)),
			issued_at = now()
		RETURNING id, user_id, actors, issued_at`,
		pq.Array([]string{actor}),
		c.PostID,
		c.UserID,
		actor,
	)
	if err != nil {
		log.Println("error", fmt.Errorf("could not insert comment notifications: %w", err))
		return
	}

	defer rows.Close()

	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.UserID, pq.Array(&n.Actors), &n.IssuedAt); err != nil {
			log.Println("error", fmt.Errorf("could not scan comment notification: %w", err))
			return
		}

		n.Type = "comment"
		n.PostID = &c.PostID

		go s.broadcastNotification(n)
	}

	if err = rows.Err(); err != nil {
		log.Println("error", fmt.Errorf("could not iterate over comment notification rows: %w", err))
		return
	}
}

func (s *Service) notifyPostMention(p Post) {
	mentions := collectMentions(p.Content)
	if len(mentions) == 0 {
		return
	}

	actors := []string{p.User.Username}
	rows, err := s.Db.Query(`
		INSERT INTO notifications (user_id, actors, type, post_id)
		SELECT users.id, $1, 'post_mention', $2 FROM users
		WHERE users.id != $3
			AND username = ANY($4)
		RETURNING id, user_id, issued_at`,
		pq.Array(actors),
		p.ID,
		p.UserID,
		pq.Array(mentions),
	)
	if err != nil {
		log.Println("error", fmt.Errorf("could not insert post mention notifications: %w", err))
		return
	}

	defer rows.Close()

	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.UserID, &n.IssuedAt); err != nil {
			log.Println("error", fmt.Errorf("could not scan post mention notification: %w", err))
			return
		}

		n.Actors = actors
		n.Type = "post_mention"
		n.PostID = &p.ID

		go s.broadcastNotification(n)
	}

	if err = rows.Err(); err != nil {
		log.Println("error", fmt.Errorf("could not iterate post mention notification rows: %w", err))
		return
	}
}

func (s *Service) notifyCommentMention(c Comment) {
	mentions := collectMentions(c.Content)
	if len(mentions) == 0 {
		return
	}

	actor := c.User.Username

	rows, err := s.Db.Query(`
		INSERT INTO notifications (user_id, actors, type, post_id)
		SELECT users.id, $1, 'comment_mention', $2 FROM users
		WHERE users.id != $3
			AND username = ANY($4)
		ON CONFLICT (user_id, type, post_id, read_at) DO UPDATE SET
			actors = array_prepend($5, array_remove(notifications.actors, $5)),
			issued_at = now()
		RETURNING id, user_id, actors, issued_at`,
		pq.Array([]string{actor}),
		c.PostID,
		c.UserID,
		pq.Array(mentions),
		actor,
	)
	if err != nil {
		log.Println("error", fmt.Errorf("could not insert comment mention notifications: %w", err))
		return
	}

	defer rows.Close()

	for rows.Next() {
		var n Notification
		if err = rows.Scan(&n.ID, &n.UserID, pq.Array(&n.Actors), &n.IssuedAt); err != nil {
			log.Println("error", fmt.Errorf("could not scan comment mention notification: %w", err))
			return
		}

		n.Type = "comment_mention"
		n.PostID = &c.PostID

		go s.broadcastNotification(n)
	}

	if err = rows.Err(); err != nil {
		log.Println("error", fmt.Errorf("could not iterate comment mention notification rows: %w", err))
		return
	}
}

// NotificationStream to receive notifications in realtime.
func (s *Service) NotificationStream(ctx context.Context) (<-chan Notification, error) {
	uid, ok := ctx.Value(KeyAuthUserID).(string)
	if !ok {
		return nil, ErrUnauthenticated
	}

	nn := make(chan Notification)
	c := &notificationClient{notifications: nn, userID: uid, ctx: ctx}
	// Signal the broker that we have a new connection
	s.BrokerRepository.notificationBroker.NewClients <- c

	go func() {
		// Listen to connection close and un-register client connection
		<-ctx.Done()
		s.BrokerRepository.notificationBroker.ClosingClients <- c
	}()
	return nn, nil
}

func (s *Service) broadcastNotification(n Notification) {
	s.BrokerRepository.notificationBroker.Notifier <- n
}
