package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type stubSearch struct{}

func (stubSearch) Search(string) ([]ai.Source, error) {
	return []ai.Source{{Title: "Rujukan", Snippet: "s", URL: "https://r"}}, nil
}

type stubChat struct{}

func (stubChat) Complete([]ai.Message) (string, error) {
	return `{"title":"Judul AI","category":"Teknologi","excerpt":"e","content":"isi"}`, nil
}

func newTestRouter(t *testing.T, env string) *gin.Engine {
	cfg := &config.Config{AppEnv: env, AppKey: "kunci-uji", DBConnection: "sqlite", AdminUsername: "admin", AdminPassword: "admin"}
	db, err := database.Setup(cfg)
	assert.NoError(t, err)
	return NewRouter(cfg, db, stubSearch{}, stubChat{})
}

type result struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
	List    []any
}

func do(r *gin.Engine, method, path, token string, body any) (int, map[string]any, []byte) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m, w.Body.Bytes()
}

func login(t *testing.T, r *gin.Engine, user, pass string) string {
	code, m, _ := do(r, "POST", "/api/auth/login", "", gin.H{"username": user, "password": pass})
	assert.Equal(t, 200, code)
	return m["data"].(map[string]any)["token"].(string)
}

func TestAlurEndToEnd(t *testing.T) {
	r := newTestRouter(t, "production")
	admin := login(t, r, "admin", "admin")
	member := login(t, r, "member", "member")

	// registrasi
	code, _, _ := do(r, "POST", "/api/auth/register", "", gin.H{"username": "citra", "email": "citra@x.id", "password": "rahasia"})
	assert.Equal(t, 201, code)

	// penulis membuat & menerbitkan artikel
	code, m, _ := do(r, "POST", "/api/posts", member, gin.H{"title": "Menjelajah Gin", "content": "isi artikel", "category": "Teknologi"})
	assert.Equal(t, 201, code)
	id := int(m["data"].(map[string]any)["id"].(float64))
	code, _, _ = do(r, "PATCH", "/api/posts/"+itoa(id)+"/toggle-publish", member, nil)
	assert.Equal(t, 200, code)

	// publik tanpa login
	code, m, _ = do(r, "GET", "/api/public/posts?q=gin", "", nil)
	assert.Equal(t, 200, code)
	assert.Len(t, m["data"], 1)
	code, _, _ = do(r, "GET", "/api/public/posts/menjelajah-gin", "", nil)
	assert.Equal(t, 200, code)
	code, _, _ = do(r, "GET", "/api/public/categories", "", nil)
	assert.Equal(t, 200, code)

	// statistik, profil, AI
	code, m, _ = do(r, "GET", "/api/posts/stats", member, nil)
	assert.Equal(t, 200, code)
	assert.Equal(t, float64(1), m["data"].(map[string]any)["total_published"])
	code, _, _ = do(r, "GET", "/api/auth/profile", member, nil)
	assert.Equal(t, 200, code)
	code, m, _ = do(r, "POST", "/api/posts/ai-generate", member, gin.H{"topic": "Go", "include_web_search": true})
	assert.Equal(t, 200, code)
	assert.Len(t, m["data"].(map[string]any)["sources"], 1)

	// admin
	code, _, _ = do(r, "GET", "/api/admin/users", admin, nil)
	assert.Equal(t, 200, code)
	code, _, _ = do(r, "GET", "/api/admin/posts", admin, nil)
	assert.Equal(t, 200, code)
	code, _, _ = do(r, "GET", "/api/admin/users/2/posts", admin, nil)
	assert.Equal(t, 200, code)
	code, _, _ = do(r, "POST", "/api/auth/logout", member, nil)
	assert.Equal(t, 200, code)
}

func TestHakAkses(t *testing.T) {
	r := newTestRouter(t, "development")
	member := login(t, r, "member", "member")
	code, _, _ := do(r, "GET", "/api/posts", "", nil)
	assert.Equal(t, 401, code)
	code, _, _ = do(r, "GET", "/api/admin/users", member, nil)
	assert.Equal(t, 403, code)
}

func TestDokumentasiDanNoRoute(t *testing.T) {
	r := newTestRouter(t, "development")

	code, m, _ := do(r, "GET", "/", "", nil)
	assert.Equal(t, 200, code)
	assert.Equal(t, "success", m["status"])

	code, _, ui := do(r, "GET", "/swagger/index.html", "", nil)
	assert.Equal(t, 200, code)
	assert.Contains(t, string(ui), "swagger")

	code, spec, _ := do(r, "GET", "/api.json", "", nil)
	assert.Equal(t, 200, code)
	assert.Equal(t, []any{"http"}, spec["schemes"])

	req := httptest.NewRequest("GET", "/api.json", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), `"https"`)

	code, m, _ = do(r, "GET", "/tidak/ada", "", nil)
	assert.Equal(t, 404, code)
	assert.Equal(t, "Endpoint not found", m["message"])
}

func TestSpesifikasiGagal(t *testing.T) {
	old := specProvider
	defer func() { specProvider = old }()
	specProvider = func(string, string) ([]byte, error) { return nil, errors.New("rusak") }
	r := newTestRouter(t, "development")
	code, m, _ := do(r, "GET", "/api.json", "", nil)
	assert.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "error", m["status"])
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestAdminLewatRegistrasiDanHakMilik(t *testing.T) {
	r := newTestRouter(t, "development")
	_, m, _ := do(r, "POST", "/api/auth/register", "", gin.H{"username": "bos", "email": "bos@x.id", "password": "rahasia", "role": "admin"})
	adm := m["data"].(map[string]any)["token"].(string)
	code, _, _ := do(r, "GET", "/api/admin/users", adm, nil)
	assert.Equal(t, 200, code)
	code, m, _ = do(r, "GET", "/api/admin/users/9999/posts", adm, nil)
	assert.Equal(t, 200, code)
	assert.Equal(t, "success", m["status"])

	member := login(t, r, "member", "member")
	code, m, _ = do(r, "POST", "/api/posts", member, gin.H{"title": "Milik Member", "content": "isi"})
	assert.Equal(t, 201, code)
	id := itoa(int(m["data"].(map[string]any)["id"].(float64)))
	assert.NotNil(t, m["data"].(map[string]any)["user_id"])

	code, _, _ = do(r, "GET", "/api/posts/"+id, adm, nil) // milik orang lain
	assert.Equal(t, 403, code)
	code, _, _ = do(r, "PATCH", "/api/posts/"+id, adm, gin.H{"title": "Dibajak"})
	assert.Equal(t, 403, code)
	code, _, _ = do(r, "DELETE", "/api/posts/99999999", member, nil)
	assert.Equal(t, 404, code)

	code, m, _ = do(r, "GET", "/api/posts/stats", member, nil)
	assert.Equal(t, 200, code)
	assert.Equal(t, float64(1), m["data"].(map[string]any)["total_articles"])
}
