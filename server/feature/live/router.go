package live

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	r.register(live)
}

// register declares every live route on a group that already authenticates.
// Tests call it too, behind a stand-in for the auth middleware, so the two
// route tables can't drift apart.
func (r *Router) register(live *gin.RouterGroup) {
	live.POST("/prefill", r.Prefill)
	live.GET("/tmdb-people", r.TmdbPeople)
	live.GET("/friends", r.Friends)

	shows := live.Group("/shows")
	shows.GET("", r.List)
	shows.GET("/:id", r.Get)
	shows.POST("", r.Create)
	shows.PUT("/:id", r.Update)
	shows.DELETE("/:id", r.Delete)
	shows.PATCH("/:id/souvenir", r.UpdateSouvenir)
	shows.POST("/:id/photos", r.AddPhoto)
	shows.PUT("/:id/photos/:photoId", r.UpdatePhoto)
	shows.DELETE("/:id/photos/:photoId", r.DeletePhoto)
	shows.POST("/:id/poster", r.UploadPoster)
	shows.POST("/:id/poster/from-photo/:photoId", r.PosterFromPhoto)
	shows.DELETE("/:id/poster", r.RemovePoster)

	people := live.Group("/people")
	people.GET("", r.People)
	people.GET("/by-tmdb/:tmdbId", r.PersonByTmdb)
	people.GET("/:id", r.Person)
	people.PUT("/:id", r.UpdatePerson)
}

// respondError maps service errors to their status. Anything unexpected is a
// 500 with a generic message: database errors are not for the client.
func respondError(c *gin.Context, err error) {
	var verr *ValidationError
	switch {
	case errors.As(err, &verr):
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: verr.Msg})
	case errors.Is(err, errTmdbUnavailable):
		c.JSON(http.StatusBadGateway, router.ErrorResponse{Error: errTmdbUnavailable.Error()})
	case errors.Is(err, errUnreadable):
		c.JSON(http.StatusBadGateway, router.ErrorResponse{Error: errUnreadable.Error()})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, router.ErrorResponse{Error: "live show not found"})
	case errors.Is(err, ErrPersonNotFound):
		c.JSON(http.StatusNotFound, router.ErrorResponse{Error: ErrPersonNotFound.Error()})
	case errors.Is(err, ErrPhotoNotFound):
		c.JSON(http.StatusNotFound, router.ErrorResponse{Error: ErrPhotoNotFound.Error()})
	default:
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "internal error"})
	}
}

// showId reads `:id`, writing the 400 itself when it is not a positive integer.
func showId(c *gin.Context) (uint, bool) {
	return positiveParam(c, "id")
}

func positiveParam(c *gin.Context, name string) (uint, bool) {
	// 31 bits: TMDB ids are stored as int, and must fit on 32-bit builds.
	id, err := strconv.ParseUint(c.Param(name), 10, 31)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: name + " must be a positive integer"})
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
	p, err := r.service.Prefill(c.Request.Context(), c.MustGet("userId").(uint), req.URL)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (r *Router) People(c *gin.Context) {
	people, err := r.service.People(c.MustGet("userId").(uint), c.Query("q"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, people)
}

func (r *Router) Person(c *gin.Context) {
	id, ok := positiveParam(c, "id")
	if !ok {
		return
	}
	p, err := r.service.Person(c.MustGet("userId").(uint), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (r *Router) UpdatePerson(c *gin.Context) {
	id, ok := positiveParam(c, "id")
	if !ok {
		return
	}
	var req domain.LivePersonUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("live: UpdatePerson ShouldBindJSON failed", "error", err)
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request body"})
		return
	}
	p, err := r.service.UpdatePerson(c.MustGet("userId").(uint), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// PersonByTmdb answers 200 with a null person rather than 404: "never seen on
// stage" is the usual case on a TMDB person page, not an error.
func (r *Router) PersonByTmdb(c *gin.Context) {
	tmdbId, ok := positiveParam(c, "tmdbId")
	if !ok {
		return
	}
	p, err := r.service.PersonByTmdb(c.MustGet("userId").(uint), int(tmdbId))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, domain.LivePersonByTmdb{Person: p})
}

func (r *Router) TmdbPeople(c *gin.Context) {
	people, err := r.service.TmdbPeople(c.MustGet("userId").(uint), c.Query("q"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, people)
}

// UpdateSouvenir is a partial update: the body is kept as raw values so a key
// sent as null (clear) differs from a key left out (unchanged).
func (r *Router) UpdateSouvenir(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request body"})
		return
	}
	body, err := decodeSouvenirBody(b)
	if err != nil {
		respondError(c, err)
		return
	}
	show, err := r.service.UpdateSouvenir(c.MustGet("userId").(uint), id, body)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, show)
}

func (r *Router) Friends(c *gin.Context) {
	friends, err := r.service.Friends(c.MustGet("userId").(uint), c.Query("q"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, friends)
}

// AddPhoto takes a multipart form: `photo` (file), `isTicket`, `caption`.
func (r *Router) AddPhoto(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	// Capped before parsing: multipart parsing would otherwise spool any
	// size to a temporary file.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPhotoBodyBytes)
	fh, err := c.FormFile("photo")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			c.JSON(http.StatusRequestEntityTooLarge, router.ErrorResponse{Error: "could not use that photo: file is too big"})
			return
		}
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "photo is required"})
		return
	}
	isTicket := false
	if v := strings.TrimSpace(c.PostForm("isTicket")); v != "" {
		if isTicket, err = strconv.ParseBool(v); err != nil {
			c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "isTicket must be true or false"})
			return
		}
	}
	f, err := fh.Open()
	if err != nil {
		slog.Error("live: AddPhoto open failed", "error", err)
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "internal error"})
		return
	}
	defer f.Close()
	p, err := r.service.AddPhoto(c.MustGet("userId").(uint), id, f, isTicket, c.PostForm("caption"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

func photoIds(c *gin.Context) (uint, uint, bool) {
	id, ok := showId(c)
	if !ok {
		return 0, 0, false
	}
	photoId, ok := positiveParam(c, "photoId")
	return id, photoId, ok
}

func (r *Router) UpdatePhoto(c *gin.Context) {
	id, photoId, ok := photoIds(c)
	if !ok {
		return
	}
	var req domain.LivePhotoUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("live: UpdatePhoto ShouldBindJSON failed", "error", err)
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "invalid request body"})
		return
	}
	p, err := r.service.UpdatePhoto(c.MustGet("userId").(uint), id, photoId, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (r *Router) DeletePhoto(c *gin.Context) {
	id, photoId, ok := photoIds(c)
	if !ok {
		return
	}
	if err := r.service.DeletePhoto(c.MustGet("userId").(uint), id, photoId); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UploadPoster takes a multipart form with the image in `poster`.
func (r *Router) UploadPoster(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	// Capped before parsing, like photos.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPhotoBodyBytes)
	fh, err := c.FormFile("poster")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			c.JSON(http.StatusRequestEntityTooLarge, router.ErrorResponse{Error: "could not use that poster image: file is too big"})
			return
		}
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: "poster is required"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		slog.Error("live: UploadPoster open failed", "error", err)
		c.JSON(http.StatusInternalServerError, router.ErrorResponse{Error: "internal error"})
		return
	}
	defer f.Close()
	show, err := r.service.UploadPoster(c.MustGet("userId").(uint), id, f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, show)
}

func (r *Router) PosterFromPhoto(c *gin.Context) {
	id, photoId, ok := photoIds(c)
	if !ok {
		return
	}
	show, err := r.service.PosterFromPhoto(c.MustGet("userId").(uint), id, photoId)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, show)
}

func (r *Router) RemovePoster(c *gin.Context) {
	id, ok := showId(c)
	if !ok {
		return
	}
	show, err := r.service.RemovePoster(c.MustGet("userId").(uint), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, show)
}
