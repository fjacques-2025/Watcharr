package theatres

import (
	"log/slog"
	"net/http"

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
	theatres := r.br.Router.Group("/theatres").
		Use(authmiddleware.AuthRequired(r.br.DB, r.br.Cfg))

	theatres.GET("/showtimes", router.WhereaboutsRequired(r.br.Cfg), r.GetShowtimes)
}

func (r *Router) GetShowtimes(c *gin.Context) {
	var req domain.TheatresRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		slog.Error("GetShowtimes: ShouldBindQuery failed!", "error", err)
		c.JSON(http.StatusBadRequest, router.ErrorResponse{
			Error: "failed to get request parameters or they are invalid",
		})
		return
	}
	resp, err := r.service.Showtimes(req, c.MustGet("userCountry").(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, router.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
