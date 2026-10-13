package live

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sbondCo/Watcharr/domain"
	"github.com/sbondCo/Watcharr/feature/auth/authmiddleware"
	"github.com/sbondCo/Watcharr/router"
)

type Router struct {
	br      *router.BaseRouter
	service *Service
}

func NewRouter(br *router.BaseRouter, service *Service) *Router {
	return &Router{br, service}
}

func (r *Router) AddRoutes() {
	live := r.br.Router.Group("/live")
	live.Use(authmiddleware.AuthRequired(r.br.Cfg))

	live.POST("/prefill", r.Prefill)

	shows := live.Group("/shows")
	shows.GET("", r.List)
	shows.GET("/:id", r.Get)
	shows.POST("", r.Create)
	shows.PUT("/:id", r.Update)
	shows.DELETE("/:id", r.Delete)
}

// respondError maps service errors to their status. Anything unexpected is a
// 500 with a generic message: database errors are not for the client.
func respondError(c *gin.Context, err error) {
	var verr *ValidationError
	switch {
	case errors.As(err, &verr):
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: verr.Msg})
	case errors.Is(err, errUnreadable):
		c.JSON(http.StatusBadGateway, router.ErrorResponse{Error: errUnreadable.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, router.ErrorResponse{Error: "live show not found"})
	default:
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "internal error"})
	}
}

// showId reads `:id`, writing the 400 itself when it is not a positive integer.
func showId(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 0)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "id must be a positive integer"})
		return 0, false
	}
	return uint(id), true
}

func bindRequest(c *gin.Context) (domain.LiveShowRequest, bool) {
	var req domain.LiveShowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("live: ShouldBindJSON failed", "error", err)
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request body"})
		return req, false
	}
	return req, true
}

func (r *Router) List(c *gin.Context) {
	shows, err := r.service.List(c.MustGet("userId").(uint))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, shows)
}

func (r *Router) Get(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	show, err := r.service.Get(c.MustGet("userId").(uint), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, show)
}

func (r *Router) Create(c *gin.Context) {
	req, ok := bindRequest(c)
	if !ok {
		return
	}
	show, err := r.service.Create(c.MustGet("userId").(uint), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, show)
}

func (r *Router) Update(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	req, ok := bindRequest(c)
	if !ok {
		return
	}
	show, err := r.service.Update(c.MustGet("userId").(uint), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, show)
}

func (r *Router) Delete(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	if err := r.service.Delete(c.MustGet("userId").(uint), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (r *Router) Prefill(c *gin.Context) {
	var req domain.LivePrefillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("live: Prefill ShouldBindJSON failed", "error", err)
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request body"})
		return
	}
	p, err := r.service.Prefill(c.Request.Context(), req.URL)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}
