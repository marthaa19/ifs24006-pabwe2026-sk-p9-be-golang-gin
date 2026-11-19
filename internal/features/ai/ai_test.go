package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var errBoom = errors.New("boom")

// ---------- searcher ----------

func srv(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestSearcherFormatBalasan(t *testing.T) {
	arr := `[{"title":"A","snippet":"s","url":"https://a"}]`
	data := `{"data":[{"title":"B","snippet":"s","url":"https://b"}]}`
	results := `{"results":[{"title":"C","snippet":"s","url":"https://c"}]}`
	many := `[{"title":"1"},{"title":"2"},{"title":"3"},{"title":"4"},{"title":"5"},{"title":"6"}]`
	for body, want := range map[string]string{arr: "A", data: "B", results: "C"} {
		s := srv(200, body)
		got, err := NewHTTPSearcher(s.URL).Search("go gin")
		s.Close()
		assert.NoError(t, err)
		assert.Equal(t, want, got[0].Title)
	}
	s := srv(200, many)
	got, _ := NewHTTPSearcher(s.URL).Search("x")
	s.Close()
	assert.Len(t, got, 5)

	s = srv(200, "bukan json")
	got, err := NewHTTPSearcher(s.URL).Search("x")
	s.Close()
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestSearcherKegagalan(t *testing.T) {
	_, err := NewHTTPSearcher("").Search("x")
	assert.Error(t, err)

	_, err = NewHTTPSearcher("http://[::1").Search("x") // URL rusak -> NewRequest gagal
	assert.Error(t, err)

	_, err = NewHTTPSearcher("http://127.0.0.1:1").Search("x") // koneksi ditolak
	assert.Error(t, err)

	s := srv(500, "")
	_, err = NewHTTPSearcher(s.URL).Search("x")
	s.Close()
	assert.Error(t, err)

	// Content-Length lebih besar dari isi -> pembacaan body gagal
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("[]"))
	}))
	_, err = NewHTTPSearcher(bad.URL).Search("x")
	bad.Close()
	assert.Error(t, err)
}

// ---------- OpenAI client ----------

func chatOK(content string) string {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}}})
	return string(b)
}

func TestOpenAIClient(t *testing.T) {
	var gotAuth, gotPath string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		_, _ = w.Write([]byte(chatOK("halo")))
	}))
	defer s.Close()

	out, err := NewOpenAIClient(s.URL+"/v1/", "kunci", "m").Complete([]Message{{Role: "user", Content: "hi"}})
	assert.NoError(t, err)
	assert.Equal(t, "halo", out)
	assert.Equal(t, "Bearer kunci", gotAuth)
	assert.Equal(t, "/v1/chat/completions", gotPath)

	// endpoint yang sudah lengkap tidak digandakan; tanpa API key tidak ada header Authorization
	_, _ = NewOpenAIClient(s.URL+"/chat/completions", "", "m").Complete(nil)
	assert.Equal(t, "", gotAuth)
	assert.Equal(t, "/chat/completions", gotPath)
}

func TestOpenAIClientKegagalan(t *testing.T) {
	_, err := NewOpenAIClient("http://[::1", "k", "m").Complete(nil)
	assert.Error(t, err)
	_, err = NewOpenAIClient("http://127.0.0.1:1", "k", "m").Complete(nil)
	assert.Error(t, err)

	for _, tc := range []struct {
		status int
		body   string
	}{
		{500, "x"}, {200, "bukan json"},
		{200, `{"error":{"message":"kuota habis"}}`}, {200, `{"choices":[]}`},
	} {
		s := srv(tc.status, tc.body)
		_, err = NewOpenAIClient(s.URL, "k", "m").Complete(nil)
		s.Close()
		assert.Error(t, err, tc.body)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("{}"))
	}))
	_, err = NewOpenAIClient(bad.URL, "k", "m").Complete(nil)
	bad.Close()
	assert.Error(t, err)
}

// ---------- service ----------

type fakeSearch struct {
	out []Source
	err error
}

func (f fakeSearch) Search(string) ([]Source, error) { return f.out, f.err }

type fakeChat struct {
	out  string
	err  error
	last []Message
}

func (f *fakeChat) Complete(m []Message) (string, error) { f.last = m; return f.out, f.err }

const draftJSON = "```json\n{\"title\":\"Judul AI\",\"category\":\"Teknologi\",\"excerpt\":\"ringkas\",\"content\":\"## Isi\\nteks\"}\n```"

func TestGenerateDenganSearch(t *testing.T) {
	chat := &fakeChat{out: draftJSON}
	svc := NewService(fakeSearch{out: []Source{{Title: "Sumber 1", Snippet: "s", URL: "https://s1"}}}, chat)
	d, err := svc.Generate(GenerateRequest{Topic: " AI di kampus ", Category: "Pendidikan", IncludeWebSearch: true})
	assert.NoError(t, err)
	assert.Equal(t, "Judul AI", d.Title)
	assert.Equal(t, "Teknologi", d.Category) // kategori dari model valid -> dipakai
	assert.Len(t, d.Sources, 1)
	assert.Contains(t, d.Content, "## Sumber Rujukan")
	assert.Contains(t, d.Content, "[Sumber 1](https://s1)")
	assert.Contains(t, chat.last[1].Content, "AI di kampus")
	assert.Contains(t, chat.last[1].Content, "https://s1")
}

func TestGenerateTanpaSearchDanKategori(t *testing.T) {
	// model memberi kategori ngawur -> pakai kategori permintaan; bila itu pun ngawur -> Umum
	raw := `{"title":"T","category":"Ngawur","excerpt":"e","content":"c"}`
	svc := NewService(fakeSearch{err: errBoom}, &fakeChat{out: raw})
	d, _ := svc.Generate(GenerateRequest{Topic: "x", Category: "Bisnis"})
	assert.Equal(t, "Bisnis", d.Category)
	assert.NotContains(t, d.Content, "Sumber Rujukan")
	d, _ = svc.Generate(GenerateRequest{Topic: "x", Category: "Ngawur"})
	assert.Equal(t, "Umum", d.Category)
	d, _ = svc.Generate(GenerateRequest{Topic: "x"})
	assert.Equal(t, "Umum", d.Category)
}

func TestGenerateSearchGagalTetapLanjut(t *testing.T) {
	svc := NewService(fakeSearch{err: errBoom}, &fakeChat{out: draftJSON})
	d, err := svc.Generate(GenerateRequest{Topic: "x", IncludeWebSearch: true})
	assert.NoError(t, err)
	assert.Empty(t, d.Sources)
}

func TestGenerateKesalahan(t *testing.T) {
	_, err := NewService(nil, nil).Generate(GenerateRequest{Topic: "   "})
	assert.ErrorIs(t, err, ErrTopicRequired)

	_, err = NewService(nil, &fakeChat{err: errBoom}).Generate(GenerateRequest{Topic: "x"})
	assert.ErrorIs(t, err, ErrUnavailable)

	for _, raw := range []string{"tanpa json", "}{", `{"title":"","content":"c"}`, `{"title":"t","content":" "}`, `{"title": 5}`} {
		_, err = NewService(nil, &fakeChat{out: raw}).Generate(GenerateRequest{Topic: "x"})
		assert.ErrorIs(t, err, ErrBadResponse, raw)
	}
}

// ---------- handler ----------

type fakeSvc struct {
	d   *Draft
	err error
}

func (f fakeSvc) Generate(GenerateRequest) (*Draft, error) { return f.d, f.err }

func doPost(svc Service, body string) (int, response.Body) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/ai", NewHandler(svc).Generate)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/ai", bytes.NewBufferString(body)))
	var b response.Body
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return w.Code, b
}

func TestHandler(t *testing.T) {
	code, b := doPost(fakeSvc{d: &Draft{Title: "T"}}, `{"topic":"x"}`)
	assert.Equal(t, 200, code)
	assert.Equal(t, "success", b.Status)

	code, _ = doPost(fakeSvc{}, `{}`)
	assert.Equal(t, 400, code)
	code, _ = doPost(fakeSvc{err: ErrTopicRequired}, `{"topic":"x"}`)
	assert.Equal(t, 400, code)
	code, _ = doPost(fakeSvc{err: ErrUnavailable}, `{"topic":"x"}`)
	assert.Equal(t, 502, code)
	code, _ = doPost(fakeSvc{err: ErrBadResponse}, `{"topic":"x"}`)
	assert.Equal(t, 502, code)
	code, b = doPost(fakeSvc{err: errBoom}, `{"topic":"x"}`)
	assert.Equal(t, 500, code)
	assert.Equal(t, "error", b.Status)
	assert.False(t, strings.Contains(b.Message, "boom"))
}
