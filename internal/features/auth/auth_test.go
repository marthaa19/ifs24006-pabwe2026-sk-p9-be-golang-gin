package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var errBoom = errors.New("boom")

func newDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	assert.NoError(t, db.AutoMigrate(&User{}))
	return db
}

func newSvc(t *testing.T) (Service, Repository, *gorm.DB) {
	db := newDB(t)
	repo := NewRepository(db)
	return NewService(repo, "kunci-uji", time.Hour), repo, db
}

// ---------- repository ----------

func TestRepository(t *testing.T) {
	db := newDB(t)
	repo := NewRepository(db)
	u := &User{Username: "ani", Email: "ani@x.id", Password: "h", Role: RoleMember}
	assert.NoError(t, repo.Create(u))
	assert.Error(t, repo.Create(&User{Username: "ani", Email: "lain@x.id", Password: "h"}))

	got, err := repo.FindByID(u.ID)
	assert.NoError(t, err)
	assert.Equal(t, "ani", got.Username)
	_, err = repo.FindByID(999)
	assert.ErrorIs(t, err, ErrNotFound)

	byName, _ := repo.FindByLogin("ani")
	byMail, _ := repo.FindByLogin("ani@x.id")
	assert.Equal(t, u.ID, byName.ID)
	assert.Equal(t, u.ID, byMail.ID)
	_, err = repo.FindByLogin("tidak-ada")
	assert.ErrorIs(t, err, ErrNotFound)

	ok, err := repo.Exists("ani", "z@z.id", 0)
	assert.NoError(t, err)
	assert.True(t, ok)
	ok, _ = repo.Exists("ani", "ani@x.id", u.ID) // dirinya sendiri dikecualikan
	assert.False(t, ok)

	u.Username = "ani2"
	assert.NoError(t, repo.Update(u))

	assert.NoError(t, db.Migrator().DropTable(&User{})) // simulasi database rusak
	_, err = repo.FindByID(1)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	_, err = repo.FindByLogin("x")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	_, err = repo.Exists("a", "b", 0)
	assert.Error(t, err)
}

// ---------- service ----------

func TestRegisterDanLogin(t *testing.T) {
	svc, _, _ := newSvc(t)
	res, err := svc.Register(RegisterRequest{Username: " budi ", Email: "Budi@X.ID", Password: "rahasia"})
	assert.NoError(t, err)
	assert.Equal(t, "budi", res.User.Username)
	assert.Equal(t, "budi@x.id", res.User.Email)
	assert.Equal(t, RoleMember, res.User.Role)
	assert.NotEmpty(t, res.Token)

	_, err = svc.Register(RegisterRequest{Username: "budi", Email: "lain@x.id", Password: "rahasia"})
	assert.ErrorIs(t, err, ErrDuplicate)

	for _, login := range []string{"budi", "budi@x.id"} {
		r, err := svc.Login(LoginRequest{Username: login, Password: "rahasia"})
		assert.NoError(t, err)
		assert.NotEmpty(t, r.Token)
	}
	_, err = svc.Login(LoginRequest{Username: "budi", Password: "salah"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = svc.Login(LoginRequest{Username: "hantu", Password: "x"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

type fakeRepo struct {
	user                                 *User
	existsVal                            bool
	existsErr, createErr, findErr, upErr error
}

func (f *fakeRepo) Create(*User) error                { return f.createErr }
func (f *fakeRepo) FindByID(uint) (*User, error)      { return f.user, f.findErr }
func (f *fakeRepo) FindByLogin(string) (*User, error) { return f.user, f.findErr }
func (f *fakeRepo) Exists(string, string, uint) (bool, error) {
	return f.existsVal, f.existsErr
}
func (f *fakeRepo) Update(*User) error { return f.upErr }

func TestServiceKesalahanRepo(t *testing.T) {
	mk := func(f *fakeRepo) Service { return NewService(f, "k", time.Hour) }
	reg := RegisterRequest{Username: "abc", Email: "a@b.id", Password: "rahasia"}

	_, err := mk(&fakeRepo{existsErr: errBoom}).Register(reg)
	assert.ErrorIs(t, err, errBoom)
	_, err = mk(&fakeRepo{createErr: errBoom}).Register(reg)
	assert.ErrorIs(t, err, errBoom)
	_, err = mk(&fakeRepo{}).Register(RegisterRequest{Username: "abc", Email: "a@b.id", Password: strings.Repeat("x", 80)})
	assert.Error(t, err) // bcrypt menolak password > 72 byte

	_, err = mk(&fakeRepo{findErr: errBoom}).Login(LoginRequest{Username: "a", Password: "b"})
	assert.ErrorIs(t, err, errBoom)

	_, err = mk(&fakeRepo{findErr: errBoom}).Profile(1)
	assert.ErrorIs(t, err, errBoom)

	up := UpdateProfileRequest{Username: "abc", Email: "a@b.id"}
	_, err = mk(&fakeRepo{findErr: ErrNotFound}).UpdateProfile(1, up)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = mk(&fakeRepo{user: &User{}, existsErr: errBoom}).UpdateProfile(1, up)
	assert.ErrorIs(t, err, errBoom)
	_, err = mk(&fakeRepo{user: &User{}, existsVal: true}).UpdateProfile(1, up)
	assert.ErrorIs(t, err, ErrDuplicate)
	_, err = mk(&fakeRepo{user: &User{}, upErr: errBoom}).UpdateProfile(1, up)
	assert.ErrorIs(t, err, errBoom)
	up.Password = strings.Repeat("x", 80)
	_, err = mk(&fakeRepo{user: &User{}}).UpdateProfile(1, up)
	assert.Error(t, err)
}

func TestServiceGagalTandatangan(t *testing.T) {
	old := signToken
	defer func() { signToken = old }()
	signToken = func(middleware.Claims, string) (string, error) { return "", errBoom }
	svc, _, _ := newSvc(t)
	_, err := svc.Register(RegisterRequest{Username: "abc", Email: "a@b.id", Password: "rahasia"})
	assert.ErrorIs(t, err, errBoom)
}

func TestProfilDanUpdate(t *testing.T) {
	svc, _, _ := newSvc(t)
	a, _ := svc.Register(RegisterRequest{Username: "ani", Email: "ani@x.id", Password: "rahasia"})
	b, _ := svc.Register(RegisterRequest{Username: "budi", Email: "budi@x.id", Password: "rahasia"})

	p, err := svc.Profile(a.User.ID)
	assert.NoError(t, err)
	assert.Equal(t, "ani", p.Username)

	u, err := svc.UpdateProfile(a.User.ID, UpdateProfileRequest{Username: "ani baru", Email: "ANI2@x.id"})
	assert.NoError(t, err)
	assert.Equal(t, "ani2@x.id", u.Email)
	_, err = svc.Login(LoginRequest{Username: "ani baru", Password: "rahasia"})
	assert.NoError(t, err) // password lama tetap berlaku

	_, err = svc.UpdateProfile(a.User.ID, UpdateProfileRequest{Username: "ani baru", Email: "ani2@x.id", Password: "kataSandiBaru"})
	assert.NoError(t, err)
	_, err = svc.Login(LoginRequest{Username: "ani baru", Password: "kataSandiBaru"})
	assert.NoError(t, err)

	_, err = svc.UpdateProfile(a.User.ID, UpdateProfileRequest{Username: "budi", Email: "ani2@x.id"})
	assert.ErrorIs(t, err, ErrDuplicate)
	_ = b
}

// ---------- handler ----------

type fakeSvc struct{ err error }

func (f fakeSvc) Register(RegisterRequest) (*AuthResult, error) { return nil, f.err }
func (f fakeSvc) Login(LoginRequest) (*AuthResult, error)       { return nil, f.err }
func (f fakeSvc) Profile(uint) (*User, error)                   { return nil, f.err }
func (f fakeSvc) UpdateProfile(uint, UpdateProfileRequest) (*User, error) {
	return nil, f.err
}

func router(svc Service, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(svc)
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	as := func(c *gin.Context) { c.Set(middleware.UserKey, &middleware.AuthUser{ID: userID}) }
	r.POST("/logout", as, h.Logout)
	r.GET("/profile", as, h.Profile)
	r.PUT("/profile", as, h.UpdateProfile)
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
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var b response.Body
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return w.Code, b
}

func TestHandlerAlurNormal(t *testing.T) {
	svc, _, _ := newSvc(t)
	r := router(svc, 1)

	code, b := call(r, "POST", "/register", gin.H{"username": "ani", "email": "ani@x.id", "password": "rahasia"})
	assert.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "success", b.Status)

	code, _ = call(r, "POST", "/login", gin.H{"username": "ani", "password": "rahasia"})
	assert.Equal(t, 200, code)
	code, _ = call(r, "POST", "/logout", "")
	assert.Equal(t, 200, code)
	code, _ = call(r, "GET", "/profile", "")
	assert.Equal(t, 200, code)
	code, _ = call(r, "PUT", "/profile", gin.H{"username": "ani", "email": "ani@x.id"})
	assert.Equal(t, 200, code)
}

func TestHandlerValidasi(t *testing.T) {
	r := router(fakeSvc{}, 1)
	for _, tc := range []struct{ m, p string }{{"POST", "/register"}, {"POST", "/login"}, {"PUT", "/profile"}} {
		code, b := call(r, tc.m, tc.p, "{rusak")
		assert.Equal(t, 400, code)
		assert.Equal(t, "fail", b.Status)
	}
}

func TestHandlerPemetaanError(t *testing.T) {
	cases := map[error]int{
		ErrDuplicate:          409,
		ErrInvalidCredentials: 401,
		ErrNotFound:           404,
		errBoom:               500,
	}
	for err, want := range cases {
		r := router(fakeSvc{err: err}, 1)
		code, _ := call(r, "POST", "/register", gin.H{"username": "abc", "email": "a@b.id", "password": "rahasia"})
		assert.Equal(t, want, code)
		code, _ = call(r, "POST", "/login", gin.H{"username": "abc", "password": "x"})
		assert.Equal(t, want, code)
		code, _ = call(r, "GET", "/profile", "")
		assert.Equal(t, want, code)
		code, _ = call(r, "PUT", "/profile", gin.H{"username": "abc", "email": "a@b.id"})
		assert.Equal(t, want, code)
	}
}
