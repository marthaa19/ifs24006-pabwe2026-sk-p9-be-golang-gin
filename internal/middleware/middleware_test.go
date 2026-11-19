package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const secret = "rahasia"

func token(t *testing.T, key string, exp time.Duration, role string) string {
	claims := Claims{UserID: 7, Username: "ani", Role: role, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(exp)),
	}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(key))
	assert.NoError(t, err)
	return s
}

func do(r *gin.Engine, method, path, auth string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	r.ServeHTTP(w, req)
	return w
}

func engine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestCORS(t *testing.T) {
	r := engine()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	w := do(r, http.MethodGet, "/x", "")
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	w = do(r, http.MethodOptions, "/x", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAuth(t *testing.T) {
	r := engine()
	r.Use(Auth(secret))
	r.GET("/me", func(c *gin.Context) { c.JSON(200, CurrentUser(c)) })

	assert.Equal(t, 401, do(r, "GET", "/me", "").Code)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer sampah").Code)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer "+token(t, "kunci-lain", time.Hour, "member")).Code)
	assert.Equal(t, 401, do(r, "GET", "/me", "Bearer "+token(t, secret, -time.Hour, "member")).Code)

	w := do(r, "GET", "/me", "Bearer "+token(t, secret, time.Hour, "member"))
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ani")
}

func TestCurrentUserKosong(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Nil(t, CurrentUser(c))
	c.Set(UserKey, "bukan-user")
	assert.Nil(t, CurrentUser(c))
}

func TestRequireRole(t *testing.T) {
	r := engine()
	r.GET("/anon", RequireRole("admin"), func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/admin", Auth(secret), RequireRole("admin"), func(c *gin.Context) { c.String(200, "ok") })

	assert.Equal(t, 403, do(r, "GET", "/anon", "").Code)
	assert.Equal(t, 403, do(r, "GET", "/admin", "Bearer "+token(t, secret, time.Hour, "member")).Code)
	assert.Equal(t, 200, do(r, "GET", "/admin", "Bearer "+token(t, secret, time.Hour, "admin")).Code)
}

func TestRateLimiter(t *testing.T) {
	r := engine()
	r.Use(RateLimiter(2, 50*time.Millisecond))
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	assert.Equal(t, 200, do(r, "GET", "/x", "").Code)
	assert.Equal(t, 200, do(r, "GET", "/x", "").Code)
	assert.Equal(t, 429, do(r, "GET", "/x", "").Code)
	time.Sleep(70 * time.Millisecond)
	assert.Equal(t, 200, do(r, "GET", "/x", "").Code)
}
