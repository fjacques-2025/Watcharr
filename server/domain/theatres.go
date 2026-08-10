package domain

import (
	"github.com/sbondCo/Watcharr/media/erakys"
	"github.com/sbondCo/Watcharr/media/telerama"
)

// One venue's screenings of a single film on the requested day.
type TheatreScreenings struct {
	// Display name of the cinema.
	Theatre   string            `json:"theatre"`
	Showtimes []erakys.Showtime `json:"showtimes"`
}

// A film screening at one or more of the user's theatres.
type TheatreFilm struct {
	// TMDB content, when we managed to identify the film. Nil otherwise, so the
	// client can still list the screening — a title we can't match is still a
	// film you could go and see, and hiding it would be a silent lie.
	Media *Media `json:"media,omitempty"`
	// Title exactly as the cinema publishes it.
	Title string `json:"title"`
	// Genre and running time as published, kept because they are what we have
	// when Media is nil.
	Genre   string `json:"genre,omitempty"`
	Runtime int    `json:"runtime,omitempty"`
	// Télérama's rating and a link to its review, when they have reviewed the
	// film. Nil otherwise — their index only covers recent releases.
	Telerama *telerama.Review `json:"telerama,omitempty"`
	// Per-venue screenings, in the order the venues are configured.
	Screenings []TheatreScreenings `json:"screenings"`
}

type TheatresRequest struct {
	// Day to list, "YYYY-MM-DD". Defaults to today when empty.
	Day string `form:"day"`
}

type TheatresResponse struct {
	// The day actually listed, "YYYY-MM-DD".
	Day string `json:"day"`
	// Names of the configured theatres, whether or not they have screenings.
	Theatres []string `json:"theatres"`
	// Films screening that day, most showtimes first.
	Films []TheatreFilm `json:"films"`
}
