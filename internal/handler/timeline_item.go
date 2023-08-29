package handler

import (
	"encoding/json"
	"mime"
	"net/http"
	"social-media/internal/service"
	"strconv"
)

type createTimelineItemInput struct {
	Content   string  `json:"content"`
	SpoilerOf *string `json:"spoilerOf"`
	NSFW      bool    `json:"nsfw"`
}

func (h *handler) createTimelineItem(w http.ResponseWriter, r *http.Request) {
	var in createTimelineItemInput

	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		h.respondErr(w, errBadRequest)
		return
	}

	ti, err := h.svc.CreateTimelineItem(r.Context(), in.Content, in.SpoilerOf, in.NSFW)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	h.respond(w, ti, http.StatusCreated)
}

func (h *handler) timeline(w http.ResponseWriter, r *http.Request) {
	if a, _, err := mime.ParseMediaType(r.Header.Get("Accept")); err == nil && a == "text/event-stream" {
		h.timelineItemStream(w, r)
		return
	}

	ctx := r.Context()
	q := r.URL.Query()
	last, _ := strconv.ParseUint(q.Get("last"), 10, 64)
	before := emptyStrPtr(q.Get("before"))
	tt, err := h.svc.Timeline(ctx, last, before)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	if tt == nil {
		tt = service.Timeline{} // non null array
	}

	// h.respond(w, tt, http.StatusOK)

	h.respond(w, paginatedRespBody{
		Items:     tt,
		EndCursor: tt.EndCursor(),
	}, http.StatusOK)
}

// Server-sent events
func (h *handler) timelineItemStream(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		h.respondErr(w, errStreamingUnsupported)
		return
	}

	ctx := r.Context()
	tt, err := h.svc.TimelineItemStream(ctx)
	if err != nil {
		h.respondErr(w, err)
		return
	}

	header := w.Header()
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("Content-Type", "text/event-stream; charset=utf-8")

	for {
		select {
		case ti := <-tt:
			h.writeSSE(w, ti)
			f.Flush()
		case <-ctx.Done():
			return
		}
	}
}
