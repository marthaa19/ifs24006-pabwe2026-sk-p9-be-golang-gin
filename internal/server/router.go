// Package server merakit seluruh handler menjadi satu Gin engine.
package server

import (
	"net/http"
	"time"

	"app/docs"
	"app/internal/config"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// TokenTTL = masa berlaku token JWT.
const TokenTTL = 24 * time.Hour

// specProvider menyediakan spesifikasi Swagger; dijadikan variabel agar kegagalan bisa diuji.
var specProvider = docs.GetSwaggerJSON

// NewRouter menyusun engine lengkap dengan seluruh rute API, Swagger UI, dan 404 JSON.
func NewRouter(cfg *config.Config, db *gorm.DB, search ai.Searcher, chat ai.Completer) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	_ = r.SetTrustedProxies(nil) // jangan percaya header proxy sembarangan
	r.Use(gin.Recovery(), middleware.CORS())

	// ---- rakit dependensi (repository -> service -> handler) ----
	authH := auth.NewHandler(auth.NewService(auth.NewRepository(db), cfg.AppKey, TokenTTL))
	postH := post.NewHandler(post.NewService(post.NewRepository(db), nil))
	aiH := ai.NewHandler(ai.NewService(search, chat))

	// ---- dokumentasi ----
	r.GET("/", func(c *gin.Context) {
		response.Success(c, http.StatusOK, "Delcom Blog API berjalan. Dokumentasi: /swagger/index.html", nil)
	})
	r.GET("/api.json", func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		spec, err := specProvider(c.Request.Host, scheme)
		if err != nil {
			response.Error(c, "Gagal membaca spesifikasi API")
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", spec)
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/api.json")))

	api := r.Group("/api", middleware.RateLimiter(600, time.Minute))
	requireLogin := middleware.Auth(cfg.AppKey)

	pub := api.Group("/public")
	pub.GET("/categories", postH.Categories)
	pub.GET("/posts", postH.PublicList)
	pub.GET("/posts/:slug", postH.PublicDetail)

	a := api.Group("/auth")
	a.POST("/register", authH.Register)
	a.POST("/login", authH.Login)
	a.POST("/logout", requireLogin, authH.Logout)
	a.GET("/profile", requireLogin, authH.Profile)
	a.PUT("/profile", requireLogin, authH.UpdateProfile)

	p := api.Group("/posts", requireLogin)
	p.GET("", postH.MyList)
	p.POST("", postH.Create)
	p.GET("/stats", postH.Stats)
	p.POST("/ai-generate", aiH.Generate)
	p.GET("/:id", postH.MyDetail)
	p.PUT("/:id", postH.Update)
	p.PATCH("/:id", postH.Update)
	p.PATCH("/:id/toggle-publish", postH.Toggle)
	p.DELETE("/:id", postH.Delete)

	adm := api.Group("/admin", requireLogin, middleware.RequireRole(auth.RoleAdmin))
	adm.GET("/users", postH.AdminUsers)
	adm.GET("/posts", postH.AdminPosts)
	adm.GET("/users/:id/posts", postH.AdminUserPosts)

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})
	return r
}
