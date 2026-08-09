package tmdb

import (
	"errors"
	"log/slog"
	"sort"
	"strings"
)

// LanguageOption is a selectable metadata language (for the TMDB_LANG setting).
type LanguageOption struct {
	Code string `json:"code"` // TMDB "language" value, e.g. "fr-FR"
	Name string `json:"name"` // Human label, e.g. "French (FR)"
}

// Languages returns TMDB's primary translation languages with readable labels,
// for the metadata-language dropdown.
func (t *TMDB) Languages() ([]LanguageOption, error) {
	prim := new([]string)
	if err := t.req("/configuration/primary_translations", map[string]string{}, &prim); err != nil {
		slog.Error("Languages: primary_translations request failed", "error", err)
		return nil, errors.New("failed to fetch languages")
	}
	langs := new([]struct {
		Iso6391     string `json:"iso_639_1"`
		EnglishName string `json:"english_name"`
	})
	if err := t.req("/configuration/languages", map[string]string{}, &langs); err != nil {
		slog.Error("Languages: languages request failed", "error", err)
		return nil, errors.New("failed to fetch languages")
	}
	nameByIso := make(map[string]string, len(*langs))
	for _, l := range *langs {
		nameByIso[l.Iso6391] = l.EnglishName
	}
	out := make([]LanguageOption, 0, len(*prim))
	for _, code := range *prim {
		lang, region := code, ""
		if i := strings.Index(code, "-"); i > 0 {
			lang, region = code[:i], code[i+1:]
		}
		name := nameByIso[lang]
		if name == "" {
			name = lang
		}
		if region != "" {
			name = name + " (" + region + ")"
		}
		out = append(out, LanguageOption{Code: code, Name: name})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out, nil
}
