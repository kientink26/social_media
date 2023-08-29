package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/matryer/way"
	"io"
	"net/http"
	"social-media/internal/service"
	"strconv"
)

type createUserInput struct {
	Email, Username string
}

func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var in createUserInput
	err := json.NewDecoder(r.Body).Decode(&in)
	if err != nil {
		h.respondErr(w, errBadRequest)
		return
	}

	ctx := r.Context()
	err = h.svc.CreateUser(ctx, in.Email, in.Username)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) toggleFollow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := way.Param(ctx, "username")

	out, err := h.svc.ToggleFollow(ctx, username)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, out, http.StatusOK)
}

func (h *handler) users(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	first, _ := strconv.ParseUint(q.Get("first"), 10, 64)
	after := q.Get("after")
	uu, err := h.svc.Users(r.Context(), search, first, after)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	if uu == nil {
		uu = []service.UserProfile{} // non null array
	}

	h.respond(w, uu, http.StatusOK)
}

func (h *handler) user(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username := way.Param(ctx, "username")
	u, err := h.svc.User(ctx, username)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, u, http.StatusOK)
}

func (h *handler) followers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	username := way.Param(ctx, "username")
	first, _ := strconv.ParseUint(q.Get("first"), 10, 64)
	after := q.Get("after")
	uu, err := h.svc.Followers(ctx, username, first, after)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	if uu == nil {
		uu = []service.UserProfile{} // non null array
	}

	h.respond(w, uu, http.StatusOK)
}

func (h *handler) followees(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	username := way.Param(ctx, "username")
	first, _ := strconv.ParseUint(q.Get("first"), 10, 64)
	after := q.Get("after")
	uu, err := h.svc.Followees(ctx, username, first, after)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	if uu == nil {
		uu = []service.UserProfile{} // non null array
	}

	h.respond(w, uu, http.StatusOK)
}

func (h *handler) updateAvatar(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, service.MaxAvatarBytes))
	if err != nil {
		h.respondErr(w, errBadRequest)
		return
	}

	avatarURL, err := h.svc.UpdateAvatar(r.Context(), bytes.NewReader(b))
	if err != nil {
		h.respondErr(w, err)
		return
	}

	fmt.Fprint(w, avatarURL)
}
