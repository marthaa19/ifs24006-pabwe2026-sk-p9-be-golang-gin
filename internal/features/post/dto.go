package post

import (
	"time"

	"app/internal/features/auth"
)

// Categories adalah daftar kategori artikel yang diizinkan.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// Post adalah model GORM tabel posts.
type Post struct {
	ID             uint      `gorm:"primaryKey"`
	UserID         uint      `gorm:"not null;index"`
	User           auth.User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Title          string    `gorm:"size:255;not null"`
	Slug           string    `gorm:"size:255;uniqueIndex;not null"`
	Category       string    `gorm:"size:100;not null;default:Umum"`
	Excerpt        string    `gorm:"type:text"`
	Content        string    `gorm:"type:text"`
	CoverImagePath string    `gorm:"type:text"`
	IsPublished    bool      `gorm:"not null;default:false"`
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PostRequest = payload POST /api/posts dan PUT /api/posts/:id.
type PostRequest struct {
	Title          string `json:"title" binding:"required,max=255"`
	Category       string `json:"category"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
	IsPublished    bool   `json:"is_published"`
}

// Author = informasi singkat penulis pada respons artikel.
type Author struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// PostResponse = bentuk artikel yang dikirim ke klien.
type PostResponse struct {
	ID             uint       `json:"id"`
	UserID         *uint      `json:"user_id,omitempty"`
	AuthorName     string     `json:"author_name"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Category       string     `json:"category"`
	Excerpt        string     `json:"excerpt"`
	Content        string     `json:"content"`
	CoverImagePath string     `json:"cover_image_path"`
	IsPublished    bool       `json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	ReadingTime    int        `json:"reading_time"`
	Author         Author     `json:"author"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Stats = tiga kartu ringkasan penulis.
type Stats struct {
	Total          int64 `json:"total"`
	TotalPublished int64 `json:"total_published"`
	TotalDraft     int64 `json:"total_draft"`
	TotalArticles  int64 `json:"total_articles"`
	TotalSelesai   int64 `json:"total_selesai"`
	BelumSelesai   int64 `json:"belum_selesai"`
}

// UpdateRequest = payload PUT/PATCH /api/posts/:id; kolom yang tidak dikirim tidak diubah.
type UpdateRequest struct {
	Title          *string `json:"title"`
	Category       *string `json:"category"`
	Excerpt        *string `json:"excerpt"`
	Content        *string `json:"content"`
	CoverImagePath *string `json:"cover_image_path"`
	IsPublished    *bool   `json:"is_published"`
}

// UserSummary = baris direktori pengguna pada panel admin.
type UserSummary struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	CreatedAt      time.Time `json:"created_at"`
}
