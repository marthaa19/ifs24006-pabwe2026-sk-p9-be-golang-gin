package post

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/features/auth"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var (
	errBoom = errors.New("boom")
	fixed   = time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	clock   = func() time.Time { return fixed }
)

func newDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	db.Exec("PRAGMA foreign_keys = ON")
	assert.NoError(t, db.AutoMigrate(&auth.User{}, &Post{}))
	db.Create(&auth.User{Username: "ani", Email: "ani@x.id", Password: "h", Role: "member"})   // id 1
	db.Create(&auth.User{Username: "budi", Email: "budi@x.id", Password: "h", Role: "member"}) // id 2
	return db
}

func newSvc(t *testing.T) (Service, Repository, *gorm.DB) {
	db := newDB(t)
	repo := NewRepository(db)
	return NewService(repo, clock), repo, db
}

func req(title string, publish bool) PostRequest {
	return PostRequest{Title: title, Category: "Teknologi", Excerpt: "ringkas", Content: "isi artikel", IsPublished: publish}
}

// ---------- fungsi murni ----------

func TestSlugify(t *testing.T) {
	assert.Equal(t, "halo-dunia", Slugify("Halo, Dunia!"))
	assert.Equal(t, "belajar-go-1-22", Slugify("  Belajar Go 1.22 -- "))
	assert.Equal(t, "artikel", Slugify("!!!"))
	assert.Equal(t, "artikel", Slugify(""))
}

func TestReadingTime(t *testing.T) {
	assert.Equal(t, 1, ReadingTime(""))
	assert.Equal(t, 1, ReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, ReadingTime(strings.Repeat("kata ", 201)))
}

func TestNormalizeCategory(t *testing.T) {
	c, err := normalizeCategory("  ")
	assert.NoError(t, err)
	assert.Equal(t, "Umum", c)
	c, _ = normalizeCategory("Bisnis")
	assert.Equal(t, "Bisnis", c)
	_, err = normalizeCategory("Olahraga")
	assert.ErrorIs(t, err, ErrInvalidCategory)
}

func TestNewServiceJamBawaan(t *testing.T) {
	s := NewService(nil, nil).(*service)
	assert.WithinDuration(t, time.Now(), s.now(), time.Second)
}

// ---------- alur nyata (service + repository + sqlite) ----------

func TestAlurPenulis(t *testing.T) {
	svc, _, _ := newSvc(t)

	a, err := svc.Create(1, req("Belajar Go", true))
	assert.NoError(t, err)
	assert.Equal(t, "belajar-go", a.Slug)
	assert.True(t, a.IsPublished)
	assert.NotNil(t, a.PublishedAt)
	assert.Equal(t, "ani", a.Author.Username)
	assert.Equal(t, 1, a.ReadingTime)

	b, _ := svc.Create(1, req("Belajar Go", false)) // judul kembar -> slug -2
	c, _ := svc.Create(1, req("Belajar Go", false))
	assert.Equal(t, "belajar-go-2", b.Slug)
	assert.Equal(t, "belajar-go-3", c.Slug)
	assert.Nil(t, b.PublishedAt)

	d, _ := svc.Create(1, PostRequest{Title: "Tanpa Kategori", Content: "x"})
	assert.Equal(t, "Umum", d.Category)
	_, err = svc.Create(1, PostRequest{Title: "Salah", Content: "x", Category: "Olahraga"})
	assert.ErrorIs(t, err, ErrInvalidCategory)

	list, _ := svc.MyList(1)
	assert.Len(t, list, 4)
	other, _ := svc.MyList(2)
	assert.Empty(t, other)

	st, _ := svc.Stats(1)
	assert.Equal(t, Stats{Total: 4, TotalPublished: 1, TotalDraft: 3}, *st)

	// update: judul berubah -> slug baru; judul sama -> slug tetap
	u, err := svc.Update(1, b.ID, req("Judul Baru", true))
	assert.NoError(t, err)
	assert.Equal(t, "judul-baru", u.Slug)
	assert.NotNil(t, u.PublishedAt)
	u, _ = svc.Update(1, b.ID, req("Judul Baru", true)) // sudah terbit: published_at tidak diubah
	assert.Equal(t, "judul-baru", u.Slug)
	u, _ = svc.Update(1, b.ID, req("Judul Baru", false))
	assert.Nil(t, u.PublishedAt)
	_, err = svc.Update(1, b.ID, PostRequest{Title: "x", Content: "x", Category: "Olahraga"})
	assert.ErrorIs(t, err, ErrInvalidCategory)
	_, err = svc.Update(2, b.ID, req("Curi", true)) // bukan pemilik
	assert.ErrorIs(t, err, ErrNotFound)

	// toggle
	tg, _ := svc.TogglePublish(1, b.ID)
	assert.True(t, tg.IsPublished)
	assert.Equal(t, &fixed, tg.PublishedAt)
	tg, _ = svc.TogglePublish(1, b.ID)
	assert.False(t, tg.IsPublished)
	_, err = svc.TogglePublish(2, b.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	// detail milik sendiri
	my, err := svc.MyDetail(1, a.ID)
	assert.NoError(t, err)
	assert.Equal(t, a.ID, my.ID)
	_, err = svc.MyDetail(2, a.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	// hapus
	assert.NoError(t, svc.Delete(1, c.ID))
	assert.ErrorIs(t, svc.Delete(1, c.ID), ErrNotFound)
}

func TestAlurPublikDanAdmin(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, _ = svc.Create(1, PostRequest{Title: "Golang Gin", Category: "Teknologi", Excerpt: "framework web", Content: "Gin itu cepat", IsPublished: true})
	_, _ = svc.Create(2, PostRequest{Title: "Resep Nasi", Category: "Gaya Hidup", Content: "masak dengan api kecil", IsPublished: true})
	_, _ = svc.Create(2, PostRequest{Title: "Draf Rahasia", Category: "Umum", Content: "belum terbit"})

	assert.Equal(t, Categories, svc.Categories())

	all, _ := svc.PublicList("", "")
	assert.Len(t, all, 2)
	byQ, _ := svc.PublicList("  GIN ", "")
	assert.Len(t, byQ, 1)
	byCat, _ := svc.PublicList("", "Gaya Hidup")
	assert.Len(t, byCat, 1)
	none, _ := svc.PublicList("gin", "Gaya Hidup")
	assert.Empty(t, none)

	d, err := svc.PublicDetail("golang-gin")
	assert.NoError(t, err)
	assert.Equal(t, "ani", d.Author.Username)
	_, err = svc.PublicDetail("draf-rahasia") // draf tidak boleh tampil publik
	assert.ErrorIs(t, err, ErrNotFound)

	users, _ := svc.AdminUsers()
	assert.Len(t, users, 2)
	assert.Equal(t, int64(1), users[0].TotalPosts)
	assert.Equal(t, int64(2), users[1].TotalPosts)
	assert.Equal(t, int64(1), users[1].TotalPublished)

	posts, _ := svc.AdminPosts()
	assert.Len(t, posts, 3)
	up, err := svc.AdminUserPosts(2)
	assert.NoError(t, err)
	assert.Len(t, up, 2)
	_, err = svc.AdminUserPosts(99)
	assert.ErrorIs(t, err, ErrNotFound)
}

// ---------- repository: kegagalan database ----------

func TestRepositoryDatabaseRusak(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)
	assert.NoError(t, repo.Create(&Post{UserID: 1, Title: "a", Slug: "a"}))
	p, _ := repo.FindOwned(1, 1)
	p.Title = "b"
	assert.NoError(t, repo.Save(p))
	assert.NoError(t, repo.Delete(p))

	// kegagalan pada pemanggilan Query ke-2 (Count terbit) -> cabang error kedua
	n := 0
	assert.NoError(t, db.Callback().Query().Before("gorm:query").Register("gagal_kedua", func(tx *gorm.DB) {
		n++
		if n == 2 {
			_ = tx.AddError(errBoom)
		}
	}))
	_, _, err := repo.Count(1)
	assert.ErrorIs(t, err, errBoom)

	assert.NoError(t, db.Migrator().DropTable(&Post{}, &auth.User{}))
	_, err = repo.ListPublic("x", "y")
	assert.Error(t, err)
	_, err = repo.FindPublicBySlug("a")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	_, err = repo.ListByUser(1)
	assert.Error(t, err)
	_, err = repo.FindOwned(1, 1)
	assert.Error(t, err)
	_, err = repo.ListAll()
	assert.Error(t, err)
	_, err = repo.SlugExists("a", 0)
	assert.Error(t, err)
	_, _, err = repo.Count(1)
	assert.Error(t, err)
	_, err = repo.ListUsers()
	assert.Error(t, err)
	_, err = repo.UserExists(1)
	assert.Error(t, err)
}

// ---------- service: kegagalan repository (fake) ----------

type fakeRepo struct {
	errs  map[string]error
	post  *Post
	taken map[string]bool
	exist bool
}

func (f *fakeRepo) e(n string) error { return f.errs[n] }

func (f *fakeRepo) ListPublic(string, string) ([]Post, error) { return nil, f.e("ListPublic") }
func (f *fakeRepo) FindPublicBySlug(string) (*Post, error)    { return f.post, f.e("FindPublicBySlug") }
func (f *fakeRepo) ListByUser(uint) ([]Post, error)           { return nil, f.e("ListByUser") }
func (f *fakeRepo) FindOwned(uint, uint) (*Post, error)       { return f.post, f.e("FindOwned") }
func (f *fakeRepo) ListAll() ([]Post, error)                  { return nil, f.e("ListAll") }
func (f *fakeRepo) Create(*Post) error                        { return f.e("Create") }
func (f *fakeRepo) Save(*Post) error                          { return f.e("Save") }
func (f *fakeRepo) Delete(*Post) error                        { return f.e("Delete") }
func (f *fakeRepo) SlugExists(s string, _ uint) (bool, error) { return f.taken[s], f.e("SlugExists") }
func (f *fakeRepo) Count(uint) (int64, int64, error)          { return 0, 0, f.e("Count") }
func (f *fakeRepo) ListUsers() ([]UserSummary, error)         { return nil, f.e("ListUsers") }
func (f *fakeRepo) UserExists(uint) (bool, error)             { return f.exist, f.e("UserExists") }

func fk(errName string, post *Post) Service {
	f := &fakeRepo{errs: map[string]error{}, post: post}
	if errName != "" {
		f.errs[errName] = errBoom
	}
	return NewService(f, clock)
}

func TestServiceKesalahanRepo(t *testing.T) {
	p := &Post{ID: 1, UserID: 1, Title: "Lama", Slug: "lama"}
	r := req("Baru", true)

	_, err := fk("ListPublic", nil).PublicList("", "")
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("FindPublicBySlug", nil).PublicDetail("x")
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("ListByUser", nil).MyList(1)
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("FindOwned", nil).MyDetail(1, 1)
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("Count", nil).Stats(1)
	assert.ErrorIs(t, err, errBoom)

	_, err = fk("SlugExists", nil).Create(1, r)
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("Create", nil).Create(1, r)
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("FindOwned", p).Create(1, r) // gagal saat memuat ulang setelah simpan
	assert.ErrorIs(t, err, errBoom)

	_, err = fk("FindOwned", nil).Update(1, 1, r)
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("SlugExists", p).Update(1, 1, r)
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("Save", p).Update(1, 1, r)
	assert.ErrorIs(t, err, errBoom)

	_, err = fk("Save", p).TogglePublish(1, 1)
	assert.ErrorIs(t, err, errBoom)
	assert.ErrorIs(t, fk("Delete", p).Delete(1, 1), errBoom)
	assert.ErrorIs(t, fk("FindOwned", nil).Delete(1, 1), errBoom)

	_, err = fk("ListUsers", nil).AdminUsers()
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("ListAll", nil).AdminPosts()
	assert.ErrorIs(t, err, errBoom)
	_, err = fk("UserExists", nil).AdminUserPosts(1)
	assert.ErrorIs(t, err, errBoom)
	_, err = NewService(&fakeRepo{errs: map[string]error{"ListByUser": errBoom}, exist: true}, clock).AdminUserPosts(1)
	assert.ErrorIs(t, err, errBoom)
}

// ---------- handler ----------

type fakeSvc struct{ err error }

func (f fakeSvc) Categories() []string                              { return []string{"Umum"} }
func (f fakeSvc) PublicList(string, string) ([]PostResponse, error) { return nil, f.err }
func (f fakeSvc) PublicDetail(string) (*PostResponse, error)        { return nil, f.err }
func (f fakeSvc) MyList(uint) ([]PostResponse, error)               { return nil, f.err }
func (f fakeSvc) MyDetail(uint, uint) (*PostResponse, error)        { return nil, f.err }
func (f fakeSvc) Stats(uint) (*Stats, error)                        { return nil, f.err }
func (f fakeSvc) Create(uint, PostRequest) (*PostResponse, error)   { return nil, f.err }
func (f fakeSvc) Update(uint, uint, PostRequest) (*PostResponse, error) {
	return nil, f.err
}
func (f fakeSvc) TogglePublish(uint, uint) (*PostResponse, error) { return nil, f.err }
func (f fakeSvc) Delete(uint, uint) error                         { return f.err }
func (f fakeSvc) AdminUsers() ([]UserSummary, error)              { return nil, f.err }
func (f fakeSvc) AdminPosts() ([]PostResponse, error)             { return nil, f.err }
func (f fakeSvc) AdminUserPosts(uint) ([]PostResponse, error)     { return nil, f.err }

func router(svc Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(svc)
	as := func(c *gin.Context) { c.Set(middleware.UserKey, &middleware.AuthUser{ID: 1}) }
	r.GET("/categories", h.Categories)
	r.GET("/public", h.PublicList)
	r.GET("/public/:slug", h.PublicDetail)
	r.GET("/posts", as, h.MyList)
	r.GET("/posts/stats", as, h.Stats)
	r.POST("/posts", as, h.Create)
	r.GET("/posts/:id", as, h.MyDetail)
	r.PUT("/posts/:id", as, h.Update)
	r.PATCH("/posts/:id/toggle", as, h.Toggle)
	r.DELETE("/posts/:id", as, h.Delete)
	r.GET("/admin/users", h.AdminUsers)
	r.GET("/admin/posts", h.AdminPosts)
	r.GET("/admin/users/:id/posts", h.AdminUserPosts)
	return r
}

func call(r *gin.Engine, method, path string, body any) (int, response.Body) {
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	w := httptest.NewRecorder()
	rq := httptest.NewRequest(method, path, &buf)
	rq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, rq)
	var b response.Body
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return w.Code, b
}

func TestHandlerAlurNormal(t *testing.T) {
	svc, _, _ := newSvc(t)
	r := router(svc)
	body := gin.H{"title": "Halo", "content": "isi", "category": "Umum", "is_published": true}

	code, _ := call(r, "POST", "/posts", body)
	assert.Equal(t, 201, code)
	for _, tc := range []struct {
		m, p string
		b    any
	}{
		{"GET", "/categories", ""}, {"GET", "/public?q=halo", ""}, {"GET", "/public/halo", ""},
		{"GET", "/posts", ""}, {"GET", "/posts/stats", ""}, {"GET", "/posts/1", ""},
		{"PUT", "/posts/1", body}, {"PATCH", "/posts/1/toggle", ""},
		{"GET", "/admin/users", ""}, {"GET", "/admin/posts", ""}, {"GET", "/admin/users/1/posts", ""},
		{"DELETE", "/posts/1", ""},
	} {
		code, b := call(r, tc.m, tc.p, tc.b)
		assert.Equal(t, 200, code, tc.m+" "+tc.p)
		assert.Equal(t, "success", b.Status)
	}
}

func TestHandlerValidasiDanIDSalah(t *testing.T) {
	r := router(fakeSvc{})
	code, _ := call(r, "POST", "/posts", gin.H{"title": ""})
	assert.Equal(t, 400, code)
	code, _ = call(r, "PUT", "/posts/1", "{rusak")
	assert.Equal(t, 400, code)
	for _, tc := range [][2]string{
		{"GET", "/posts/abc"}, {"PUT", "/posts/abc"}, {"PATCH", "/posts/abc/toggle"},
		{"DELETE", "/posts/abc"}, {"GET", "/admin/users/abc/posts"},
	} {
		code, b := call(r, tc[0], tc[1], gin.H{"title": "a", "content": "b"})
		assert.Equal(t, 400, code, tc[1])
		assert.Equal(t, "fail", b.Status)
	}
}

func TestHandlerPemetaanError(t *testing.T) {
	body := gin.H{"title": "a", "content": "b"}
	for err, want := range map[error]int{ErrNotFound: 404, ErrInvalidCategory: 422, errBoom: 500} {
		r := router(fakeSvc{err: err})
		for _, tc := range []struct{ m, p string }{
			{"GET", "/public"}, {"GET", "/public/x"}, {"GET", "/posts"}, {"GET", "/posts/stats"},
			{"GET", "/posts/1"}, {"POST", "/posts"}, {"PUT", "/posts/1"}, {"PATCH", "/posts/1/toggle"},
			{"DELETE", "/posts/1"}, {"GET", "/admin/users"}, {"GET", "/admin/posts"}, {"GET", "/admin/users/1/posts"},
		} {
			code, _ := call(r, tc.m, tc.p, body)
			assert.Equal(t, want, code, tc.m+" "+tc.p)
		}
	}
}
