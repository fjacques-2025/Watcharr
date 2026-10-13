package live

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// testEngine wires the handlers behind a stand-in for AuthRequired, which
// would need a signed token: these tests are about the HTTP contract the
// frontend relies on, not about auth.
func testEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := &Router{service: newTestService(t)}
	e := gin.New()
	live := e.Group("/api/live", func(c *gin.Context) { c.Set("userId", uint(1)) })
	live.POST("/prefill", r.Prefill)
	g := live.Group("/shows")
	g.GET("", r.List)
	g.GET("/:id", r.Get)
	g.POST("", r.Create)
	g.PUT("/:id", r.Update)
	g.DELETE("/:id", r.Delete)
	return e
}

func do(e *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestHTTPContract(t *testing.T) {
	e := testEngine(t)

	if w := do(e, "GET", "/api/live/shows", ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s", w.Code, w.Body)
	}

	w := do(e, "POST", "/api/live/shows",
		`{"title":"A","venue":"V","url":"","description":"","date":"2026-11-05T20:00:00+01:00"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var obj map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// No poster: the key is omitted, posterUrl is "".
	want := "createdAt,date,description,id,missed,posterUrl,state,title,updatedAt,url,venue"
	if strings.Join(keys, ",") != want {
		t.Errorf("keys = %v, want %s", keys, want)
	}
	if obj["date"] != "2026-11-05T19:00:00Z" {
		t.Errorf("date = %v", obj["date"])
	}

	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/api/live/shows/1", "", 200},
		{"GET", "/api/live/shows/0", "", 400},
		{"GET", "/api/live/shows/-1", "", 400},
		{"GET", "/api/live/shows/abc", "", 400},
		{"GET", "/api/live/shows/42", "", 404},
		{"POST", "/api/live/shows", `{"title":"","date":"2026-11-05T20:00:00Z"}`, 400},
		{"POST", "/api/live/shows", `not json`, 400},
		{"PUT", "/api/live/shows/1", `{"title":"B","date":"2026-11-05T20:00:00Z","missed":true}`, 200},
		{"PUT", "/api/live/shows/42", `{"title":"B","date":"2026-11-05T20:00:00Z"}`, 404},
		{"DELETE", "/api/live/shows/1", "", 204},
		{"DELETE", "/api/live/shows/1", "", 404},
	} {
		if w := do(e, tc.method, tc.path, tc.body); w.Code != tc.code {
			t.Errorf("%s %s: got %d, want %d (%s)", tc.method, tc.path, w.Code, tc.code, w.Body)
		}
	}
}
