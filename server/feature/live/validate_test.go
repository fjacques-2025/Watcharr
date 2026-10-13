package live

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sbondCo/Watcharr/domain"
)

func validRequest() domain.LiveShowRequest {
	return domain.LiveShowRequest{
		Title:       "  Le Misanthrope  ",
		Venue:       " La Criée ",
		URL:         " https://theatre-lacriee.com/programmation/le-misanthrope ",
		Description: " Molière. ",
		Date:        "2026-11-05T20:00:00+01:00",
	}
}

func TestValidateTrimsAndNormalizes(t *testing.T) {
	f, err := validate(validRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Title != "Le Misanthrope" || f.Venue != "La Criée" || f.Description != "Molière." ||
		f.URL != "https://theatre-lacriee.com/programmation/le-misanthrope" {
		t.Errorf("fields not trimmed: %+v", f)
	}
	want := time.Date(2026, 11, 5, 19, 0, 0, 0, time.UTC)
	if !f.Date.Equal(want) || f.Date.Location() != time.UTC {
		t.Errorf("date = %v, want %v in UTC", f.Date, want)
	}
}

func TestValidateAcceptsOptionalFieldsEmpty(t *testing.T) {
	r := domain.LiveShowRequest{Title: "X", Date: "2026-11-05T20:00:00Z"}
	if _, err := validate(r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(r *domain.LiveShowRequest)
		msg    string
	}{
		{"empty title", func(r *domain.LiveShowRequest) { r.Title = "" }, "title is required"},
		{"blank title", func(r *domain.LiveShowRequest) { r.Title = "   " }, "title is required"},
		{"long title", func(r *domain.LiveShowRequest) { r.Title = strings.Repeat("a", 301) }, "title"},
		{"long venue", func(r *domain.LiveShowRequest) { r.Venue = strings.Repeat("a", 201) }, "venue"},
		{"long description", func(r *domain.LiveShowRequest) { r.Description = strings.Repeat("a", 5001) }, "description"},
		{"missing date", func(r *domain.LiveShowRequest) { r.Date = "" }, "date is required"},
		{"date without time", func(r *domain.LiveShowRequest) { r.Date = "2026-11-05" }, "RFC3339"},
		{"date without offset", func(r *domain.LiveShowRequest) { r.Date = "2026-11-05T20:00:00" }, "RFC3339"},
		{"garbage date", func(r *domain.LiveShowRequest) { r.Date = "tomorrow" }, "RFC3339"},
		{"relative url", func(r *domain.LiveShowRequest) { r.URL = "/programmation/x" }, "url"},
		{"no scheme", func(r *domain.LiveShowRequest) { r.URL = "theatre-lacriee.com" }, "url"},
		{"ftp url", func(r *domain.LiveShowRequest) { r.URL = "ftp://example.com/x" }, "url"},
		{"javascript url", func(r *domain.LiveShowRequest) { r.URL = "javascript:alert(1)" }, "url"},
		{"no host", func(r *domain.LiveShowRequest) { r.URL = "https://" }, "url"},
		{"relative poster", func(r *domain.LiveShowRequest) { r.PosterURL = "/img/a.jpg" }, "posterUrl"},
		{"data poster", func(r *domain.LiveShowRequest) { r.PosterURL = "data:image/png;base64,AAAA" }, "posterUrl"},
		{"file poster", func(r *domain.LiveShowRequest) { r.PosterURL = "file:///etc/passwd" }, "posterUrl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRequest()
			tt.mutate(&r)
			_, err := validate(r)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("want a *ValidationError, got %v", err)
			}
			if !strings.Contains(verr.Msg, tt.msg) {
				t.Errorf("message %q does not mention %q", verr.Msg, tt.msg)
			}
		})
	}
}

// Limits count characters: an accented title at the limit must pass even
// though it is longer in bytes.
func TestValidateLimitsCountCharacters(t *testing.T) {
	r := validRequest()
	r.Title = strings.Repeat("é", 300)
	r.Venue = strings.Repeat("é", 200)
	r.Description = strings.Repeat("é", 5000)
	if _, err := validate(r); err != nil {
		t.Fatalf("limits should count characters, got %v", err)
	}
}
