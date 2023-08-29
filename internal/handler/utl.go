package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"social-media/internal/service"
	"syscall"
)

var (
	errBadRequest           = errors.New("bad request")
	errStreamingUnsupported = errors.New("streaming unsupported")
	errTeaPot               = errors.New("i am a teapot")
	errInvalidTargetURL     = service.InvalidArgumentError("invalid target URL")
	errOauthTimeout         = errors.New("oauth timeout")
	errEmailNotVerified     = errors.New("email not verified")
	errEmailNotProvided     = errors.New("email not provided")
	errServiceUnavailable   = errors.New("service unavailable")
)

type paginatedRespBody struct {
	Items       interface{} `json:"items"`
	StartCursor *string     `json:"startCursor"`
	EndCursor   *string     `json:"endCursor"`
}

func (h *handler) respond(w http.ResponseWriter, v interface{}, statusCode int) {
	b, err := json.Marshal(v)
	if err != nil {
		h.respondErr(w, fmt.Errorf("could not json marshal http response body: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_, err = w.Write(b)
	if err != nil && !errors.Is(err, syscall.EPIPE) && !errors.Is(err, context.Canceled) {
		h.logger.Println(fmt.Errorf("could not write down http response: %w", err))
	}
}

func (h *handler) respondErr(w http.ResponseWriter, err error) {
	statusCode := err2code(err)
	if statusCode == http.StatusInternalServerError {
		if !errors.Is(err, context.Canceled) {
			h.logger.Println("err", err)
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Error(w, err.Error(), statusCode)
}

func err2code(err error) int {
	if err == nil {
		return http.StatusOK
	}

	switch {
	case err == errBadRequest ||
		err == errOauthTimeout ||
		err == errEmailNotVerified ||
		err == errEmailNotProvided:
		return http.StatusBadRequest
	case err == errStreamingUnsupported:
		return http.StatusExpectationFailed
	case err == errTeaPot:
		return http.StatusTeapot
	case errors.Is(err, service.ErrInvalidArgument):
		return http.StatusUnprocessableEntity
	case errors.Is(err, service.ErrNotFound) ||
		errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, service.ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, service.ErrPermissionDenied):
		return http.StatusForbidden
	case err == service.ErrUnauthenticated || errors.Is(err, service.ErrUnauthenticated):
		return http.StatusUnauthorized
	case errors.Is(err, service.ErrUnimplemented):
		return http.StatusNotImplemented
	case errors.Is(err, service.ErrGone):
		return http.StatusGone
	case err == errServiceUnavailable:
		return http.StatusServiceUnavailable
	}

	return http.StatusInternalServerError
}

func emptyStrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *handler) writeSSE(w io.Writer, v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		h.logger.Println("err", fmt.Errorf("could not json marshal sse data: %w", err))
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", b)
}
