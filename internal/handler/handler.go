package handler

import (
	"github.com/matryer/way"
	"log"
	"net/http"
	"social-media/internal/service"
)

type handler struct {
	svc    *service.Service
	logger *log.Logger
}

func New(s *service.Service, logger *log.Logger) http.Handler {
	h := &handler{s, logger}

	api := way.NewRouter()
	api.HandleFunc(http.MethodPost, "/login", h.login)
	api.HandleFunc(http.MethodGet, "/token", h.token)
	api.HandleFunc(http.MethodGet, "/otp", h.otp)
	api.HandleFunc(http.MethodPost, "/users", h.createUser)
	api.HandleFunc(http.MethodGet, "/auth_user", h.authUser)
	api.HandleFunc(http.MethodGet, "/users/:username", h.user)
	api.HandleFunc(http.MethodGet, "/users", h.users)
	api.HandleFunc(http.MethodPost, "/users/:username/toggle_follow", h.toggleFollow)
	api.HandleFunc(http.MethodGet, "/users/:username/followers", h.followers)
	api.HandleFunc(http.MethodGet, "/users/:username/followees", h.followees)
	api.HandleFunc(http.MethodPut, "/auth_user/avatar", h.updateAvatar)
	api.HandleFunc(http.MethodPost, "/timeline", h.createTimelineItem)
	api.HandleFunc(http.MethodPost, "/posts/:post_id/toggle_like", h.togglePostLike)
	api.HandleFunc(http.MethodGet, "/users/:username/posts", h.posts)
	api.HandleFunc(http.MethodGet, "/posts/:post_id", h.post)
	api.HandleFunc(http.MethodGet, "/timeline", h.timeline)
	api.HandleFunc(http.MethodPost, "/posts/:post_id/comments", h.createComment)
	api.HandleFunc(http.MethodGet, "/posts/:post_id/comments", h.comments)
	api.HandleFunc(http.MethodPost, "/comments/:comment_id/toggle_like", h.toggleCommentLike)
	api.HandleFunc(http.MethodGet, "/notifications", h.notifications)
	api.HandleFunc(http.MethodPost, "/notifications/:notification_id/mark_as_read", h.markNotificationAsRead)
	api.HandleFunc(http.MethodPost, "/mark_notifications_as_read", h.markNotificationsAsRead)
	api.HandleFunc(http.MethodGet, "/has_unread_notifications", h.hasUnreadNotifications)
	api.HandleFunc(http.MethodPost, "/posts/:post_id/toggle_subscription", h.togglePostSubscription)

	r := way.NewRouter()
	r.Handle("*", "/api...", http.StripPrefix("/api", h.withAuth(api)))
	r.Handle(http.MethodGet, "/...", withoutCache(h.staticHandler()))

	return r
}
