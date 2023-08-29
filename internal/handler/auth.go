package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"social-media/internal/service"
	"strings"
)

type loginInput struct {
	Email string
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in loginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.respondErr(w, errBadRequest)
		return
	}

	out, err := h.svc.Login(r.Context(), in.Email)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, out, http.StatusOK)
}

func (h *handler) token(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Token(r.Context())
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, out, http.StatusOK)
}

func (h *handler) authUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.AuthUser(r.Context())
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, u, http.StatusOK)
}

func (h *handler) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")
		otp := strings.TrimSpace(r.URL.Query().Get("otp"))
		var token string
		if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
			token = a[7:]
		}

		if token == "" && otp == "" {
			next.ServeHTTP(w, r)
			return
		}

		var uid string
		var err error
		if otp != "" {
			uid, err = h.svc.AuthUserIDFromOTP(otp, r.Context())
			if err == nil {
				err = h.svc.DeleteAllOTPForUser(uid, r.Context())
			}
		} else {
			uid, err = h.svc.AuthUserIDFromToken(token)
		}

		if err != nil {
			h.respondErr(w, err)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, service.KeyAuthUserID, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *handler) otp(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.OTP(r.Context())
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, out, http.StatusOK)
}
