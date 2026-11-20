package post

import (
	"errors"
	"strings"

	"app/internal/features/auth"

	"gorm.io/gorm"
)

// ErrNotFound dipakai bila artikel/pengguna tidak ada (atau bukan milik pemanggil).
var ErrNotFound = errors.New("data tidak ditemukan")

// Repository mengisolasi seluruh query tabel posts.
type Repository interface {
	ListPublic(q, category string) ([]Post, error)
	FindPublicBySlug(slug string) (*Post, error)
	ListByUser(userID uint) ([]Post, error)
	FindOwned(id, userID uint) (*Post, error)
	ListAll() ([]Post, error)
	Create(p *Post) error
	Save(p *Post) error
	Delete(p *Post) error
	SlugExists(slug string, exceptID uint) (bool, error)
	Count(userID uint) (total, published int64, err error)
	ListUsers() ([]UserSummary, error)
	UserExists(id uint) (bool, error)
	PostExists(id uint) (bool, error)
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

func (r *gormRepository) withAuthor() *gorm.DB { return r.db.Preload("User") }

func (r *gormRepository) ListPublic(q, category string) ([]Post, error) {
	tx := r.withAuthor().Where("is_published = ?", true)
	if category != "" {
		tx = tx.Where("category = ?", category)
	}
	if q = strings.ToLower(strings.TrimSpace(q)); q != "" {
		like := "%" + q + "%"
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ?", like, like, like)
	}
	var out []Post
	err := tx.Order("published_at DESC, id DESC").Find(&out).Error
	return out, err
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (r *gormRepository) FindPublicBySlug(slug string) (*Post, error) {
	var p Post
	if err := r.withAuthor().Where("slug = ? AND is_published = ?", slug, true).First(&p).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *gormRepository) ListByUser(userID uint) ([]Post, error) {
	var out []Post
	err := r.withAuthor().Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) FindOwned(id, userID uint) (*Post, error) {
	var p Post
	if err := r.withAuthor().Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, notFound(err)
	}
	return &p, nil
}

func (r *gormRepository) ListAll() ([]Post, error) {
	var out []Post
	err := r.withAuthor().Order("created_at DESC, id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) Create(p *Post) error { return r.db.Create(p).Error }
func (r *gormRepository) Save(p *Post) error   { return r.db.Omit("User").Save(p).Error }
func (r *gormRepository) Delete(p *Post) error { return r.db.Delete(p).Error }

func (r *gormRepository) SlugExists(slug string, exceptID uint) (bool, error) {
	var n int64
	err := r.db.Model(&Post{}).Where("slug = ? AND id <> ?", slug, exceptID).Count(&n).Error
	return n > 0, err
}

func (r *gormRepository) Count(userID uint) (int64, int64, error) {
	var total, published int64
	if err := r.db.Model(&Post{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	err := r.db.Model(&Post{}).Where("user_id = ? AND is_published = ?", userID, true).Count(&published).Error
	return total, published, err
}

func (r *gormRepository) ListUsers() ([]UserSummary, error) {
	var out []UserSummary
	err := r.db.Model(&auth.User{}).
		Select("users.id, users.username, users.email, users.role, users.created_at, " +
			"COUNT(posts.id) AS total_posts, " +
			"COUNT(posts.id) AS total_articles, " +
			"COALESCE(SUM(CASE WHEN posts.is_published THEN 1 ELSE 0 END), 0) AS total_published").
		Joins("LEFT JOIN posts ON posts.user_id = users.id").
		Group("users.id").Order("users.id").
		Scan(&out).Error
	return out, err
}

func (r *gormRepository) UserExists(id uint) (bool, error) {
	var n int64
	err := r.db.Model(&auth.User{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}

func (r *gormRepository) PostExists(id uint) (bool, error) {
	var n int64
	err := r.db.Model(&Post{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}
