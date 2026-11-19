// Package middleware berisi CORS, autentikasi JWT, pembatas role, dan rate limiter.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// UserKey adalah kunci Gin context tempat data pengguna login disimpan.
const UserKey = "user"

// Claims adalah isi (payload) token JWT.
type Claims struct {
	UserID   uint   `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// AuthUser adalah ringkasan pengguna yang disuntikkan ke context setelah token valid.
type AuthUser struct {
	ID       uint
	Username string
	Role     string
}

// CurrentUser mengambil pengguna login dari context (nil bila belum login).
func CurrentUser(c *gin.Context) *AuthUser {
	v, ok := c.Get(UserKey)
	if !ok {
		return nil
	}
	u, _ := v.(*AuthUser)
	return u
}

// CORS mengizinkan frontend (origin lain) memanggil API ini.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth memverifikasi header "Authorization: Bearer <token>".
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.AbortFail(c, http.StatusUnauthorized, "Token autentikasi tidak ditemukan")
			return
		}
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), claims, func(*jwt.Token) (any, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !token.Valid {
			response.AbortFail(c, http.StatusUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
			return
		}
		c.Set(UserKey, &AuthUser{ID: claims.UserID, Username: claims.Username, Role: claims.Role})
		c.Next()
	}
}

// RequireRole hanya meloloskan pengguna dengan salah satu role yang diizinkan.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u != nil {
			for _, r := range roles {
				if u.Role == r {
					c.Next()
					return
				}
			}
		}
		response.AbortFail(c, http.StatusForbidden, "Anda tidak memiliki hak akses")
	}
}

// RateLimiter membatasi jumlah request per IP dalam satu jendela waktu.
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	type bucket struct {
		count int
		reset time.Time
	}
	buckets := map[string]*bucket{}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()
		mu.Lock()
		b, ok := buckets[ip]
		if !ok || now.After(b.reset) {
			b = &bucket{reset: now.Add(window)}
			buckets[ip] = b
		}
		b.count++
		over := b.count > limit
		mu.Unlock()
		if over {
			response.AbortFail(c, http.StatusTooManyRequests, "Terlalu banyak permintaan, coba lagi nanti")
			return
		}
		c.Next()
	}
}
