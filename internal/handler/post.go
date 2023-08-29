package handler

import (
	"github.com/matryer/way"
	"net/http"
	"social-media/internal/service"
	"strconv"
)

func (h *handler) togglePostLike(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID := way.Param(ctx, "post_id")
	out, err := h.svc.TogglePostLike(ctx, postID)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, out, http.StatusOK)
}

func (h *handler) posts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	last, _ := strconv.ParseUint(q.Get("last"), 10, 64)
	before := emptyStrPtr(q.Get("before"))
	username := way.Param(ctx, "username")

	pp, err := h.svc.Posts(ctx, username, last, before)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	if pp == nil {
		pp = service.Posts{} // non null array
	}

	// h.respond(w, pp, http.StatusOK)

	h.respond(w, paginatedRespBody{
		Items:     pp,
		EndCursor: pp.EndCursor(),
	}, http.StatusOK)
}

func (h *handler) post(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID := way.Param(ctx, "post_id")
	p, err := h.svc.Post(ctx, postID)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, p, http.StatusOK)
}

func (h *handler) togglePostSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postID := way.Param(ctx, "post_id")
	out, err := h.svc.TogglePostSubscription(ctx, postID)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, out, http.StatusOK)
}
