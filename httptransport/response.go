package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	apperrors "github.com/gxmmx/compage-go/errors"
)

// -----------------------------------------------------------------------------
// Concrete types
// -----------------------------------------------------------------------------

type apiMeta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	NextPage   int `json:"next_page,omitempty"`
	TotalCount int `json:"total_count,omitempty"`
}

type apiError struct {
	Code int `json:"code"`
	Data any `json:"details,omitempty"`
}

type apiResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message"`
	Data    any       `json:"data,omitempty"`
	Meta    *apiMeta  `json:"meta,omitempty"`
	Error   *apiError `json:"error,omitempty"`
}

type apiResponseHandler struct {
	resp  apiResponse
	w     http.ResponseWriter
	r     *http.Request
	debug bool
}

// -----------------------------------------------------------------------------
// Constructors
// -----------------------------------------------------------------------------

func NewResponse(w http.ResponseWriter, r *http.Request, debug bool) *apiResponseHandler {
	return &apiResponseHandler{
		resp:  apiResponse{},
		w:     w,
		r:     r,
		debug: debug,
	}
}

// -----------------------------------------------------------------------------
// Internal
// -----------------------------------------------------------------------------

func (h *apiResponseHandler) respond(code int, err error) {
	h.w.Header().Set("Content-Type", "application/json")
	h.w.WriteHeader(code)

	// Write error to logging response writer if available
	if lrw, ok := h.w.(*loggingResponseWriter); ok {
		lrw.writeError(err)
	}

	// TODO: validate json
	json.NewEncoder(h.w).Encode(h.resp)
}

func (h *apiResponseHandler) getPageAndPerPage() (int, int) {
	query := h.r.URL.Query()
	page := 1
	perPage := 10

	if p := query.Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			if parsed != 0 {
				page = parsed
			}
		}
	}

	if pp := query.Get("perpage"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil {
			if parsed != 0 {
				perPage = parsed
			}
		}
	}

	return page, perPage
}

// -----------------------------------------------------------------------------
// Response functions
// -----------------------------------------------------------------------------

// -------------------------------------
// Generic success and error
// -------------------------------------

func (h *apiResponseHandler) Success(msg string, code int, data any) {
	h.resp.Success = true
	h.resp.Message = msg
	h.resp.Data = data
	h.respond(code, nil)
}

func (h *apiResponseHandler) Error(err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		h.resp.Success = false
		if appErr.Kind == apperrors.KindInternal {
			h.resp.Message = "internal error"
		} else {
			h.resp.Message = appErr.Message
		}
		h.resp.Error = &apiError{
			Code: appErr.HTTPStatus(),
			Data: appErr.ToJSON(h.debug),
		}
		h.respond(appErr.HTTPStatus(), appErr)
		return
	}

	h.resp.Success = false
	h.resp.Message = err.Error()
	h.resp.Error = &apiError{
		Code: http.StatusInternalServerError,
		Data: err,
	}
	h.respond(http.StatusInternalServerError, err)
}

// -------------------------------------
// Predefined CRUD item functions
// -------------------------------------

func (h *apiResponseHandler) Created(name string, data any) {
	h.resp.Success = true
	h.resp.Message = name + " created"
	h.resp.Data = data
	h.respond(http.StatusCreated, nil)
}

func (h *apiResponseHandler) Updated(name string, data any) {
	h.resp.Success = true
	h.resp.Message = name + " updated"
	h.resp.Data = data
	h.respond(http.StatusOK, nil)
}

func (h *apiResponseHandler) Deleted(name string, data any) {
	h.resp.Success = true
	h.resp.Message = name + " deleted"
	h.resp.Data = data
	h.respond(http.StatusOK, nil)
}

func (h *apiResponseHandler) ReadItem(name string, data any) {
	h.resp.Success = true
	h.resp.Message = name + " data"
	h.resp.Data = data
	h.resp.Meta = &apiMeta{
		Page:       1,
		PerPage:    1,
		NextPage:   0,
		TotalCount: 1,
	}
	h.respond(http.StatusOK, nil)
}

func (h *apiResponseHandler) ReadList(name string, data []any) {
	totalCount := len(data)
	page, perPage := h.getPageAndPerPage()
	startIndex := (page - 1) * perPage
	endIndex := startIndex + perPage

	if startIndex > totalCount {
		startIndex = totalCount
	}
	if endIndex > totalCount {
		endIndex = totalCount
	}

	pagedData := data[startIndex:endIndex]

	h.resp.Success = true
	h.resp.Message = name + " data list"
	h.resp.Data = pagedData
	h.resp.Meta = &apiMeta{
		Page:       page,
		PerPage:    perPage,
		TotalCount: totalCount,
		NextPage:   calculateNextPage(page, perPage, totalCount),
	}
	h.respond(http.StatusOK, nil)
}
