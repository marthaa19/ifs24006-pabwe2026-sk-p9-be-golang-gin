package post

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Kesalahan domain.
var (
	ErrInvalidCategory = errors.New("kategori tidak valid")
	ErrForbidden       = errors.New("anda tidak berhak mengakses artikel ini")
)

// WordsPerMinute = kecepatan baca rata-rata untuk estimasi waktu baca.
const WordsPerMinute = 200

// Service berisi logika bisnis artikel.
type Service interface {
	Categories() []string
	PublicList(q, category string) ([]PostResponse, error)
	PublicDetail(slug string) (*PostResponse, error)
	MyList(userID uint) ([]PostResponse, error)
	MyDetail(userID, id uint) (*PostResponse, error)
	Stats(userID uint) (*Stats, error)
	Create(userID uint, req PostRequest) (*PostResponse, error)
	Update(userID, id uint, req PostRequest) (*PostResponse, error)
	TogglePublish(userID, id uint) (*PostResponse, error)
	Delete(userID, id uint) error
	AdminUsers() ([]UserSummary, error)
	AdminPosts() ([]PostResponse, error)
	AdminUserPosts(userID uint) ([]PostResponse, error)
}

type service struct {
	repo Repository
	now  func() time.Time
}

// NewService membuat Service. Parameter now boleh nil (memakai jam sistem).
func NewService(repo Repository, now func() time.Time) Service {
	if now == nil {
		now = time.Now
	}
	return &service{repo: repo, now: now}
}

// Slugify mengubah judul menjadi slug URL: huruf kecil, kata dipisah tanda hubung.
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if s == "" {
		return "artikel"
	}
	return s
}

// ReadingTime menghitung estimasi menit baca (kata / 200, dibulatkan ke atas, minimal 1).
func ReadingTime(content string) int {
	words := len(strings.Fields(content))
	minutes := (words + WordsPerMinute - 1) / WordsPerMinute
	if minutes < 1 {
		return 1
	}
	return minutes
}

func (s *service) uniqueSlug(title string, exceptID uint) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		taken, err := s.repo.SlugExists(slug, exceptID)
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func normalizeCategory(c string) (string, error) {
	c = strings.TrimSpace(c)
	if c == "" {
		return "Umum", nil
	}
	for _, k := range Categories {
		if k == c {
			return c, nil
		}
	}
	return "", ErrInvalidCategory
}

func toResponse(p Post) PostResponse {
	owner := p.UserID
	return PostResponse{
		ID: p.ID, UserID: &owner, AuthorName: p.User.Username, Title: p.Title, Slug: p.Slug, Category: p.Category, Excerpt: p.Excerpt,
		Content: p.Content, CoverImagePath: p.CoverImagePath, IsPublished: p.IsPublished,
		PublishedAt: p.PublishedAt, ReadingTime: ReadingTime(p.Content),
		Author:    Author{ID: p.UserID, Username: p.User.Username},
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// owned memuat artikel milik userID: 404 bila artikel tidak ada, 403 bila milik orang lain.
func (s *service) owned(userID, id uint) (*Post, error) {
	p, err := s.repo.FindOwned(id, userID)
	if errors.Is(err, ErrNotFound) {
		exists, xerr := s.repo.PostExists(id)
		if xerr != nil {
			return nil, xerr
		}
		if exists {
			return nil, ErrForbidden
		}
	}
	return p, err
}

// publicResponses menyembunyikan user_id pada respons publik.
func publicResponses(list []PostResponse) []PostResponse {
	for i := range list {
		list[i].UserID = nil
	}
	return list
}

func toResponses(list []Post) []PostResponse {
	out := make([]PostResponse, 0, len(list))
	for _, p := range list {
		out = append(out, toResponse(p))
	}
	return out
}

func (s *service) setPublished(p *Post, publish bool) {
	switch {
	case publish && !p.IsPublished:
		t := s.now()
		p.PublishedAt = &t
	case !publish:
		p.PublishedAt = nil
	}
	p.IsPublished = publish
}

func (s *service) Categories() []string { return Categories }

func (s *service) PublicList(q, category string) ([]PostResponse, error) {
	list, err := s.repo.ListPublic(q, category)
	return publicResponses(toResponses(list)), err
}

func (s *service) PublicDetail(slug string) (*PostResponse, error) {
	p, err := s.repo.FindPublicBySlug(slug)
	if err != nil {
		return nil, err
	}
	r := toResponse(*p)
	r.UserID = nil
	return &r, nil
}

func (s *service) MyList(userID uint) ([]PostResponse, error) {
	list, err := s.repo.ListByUser(userID)
	return toResponses(list), err
}

func (s *service) MyDetail(userID, id uint) (*PostResponse, error) {
	p, err := s.owned(userID, id)
	if err != nil {
		return nil, err
	}
	r := toResponse(*p)
	return &r, nil
}

func (s *service) Stats(userID uint) (*Stats, error) {
	total, published, err := s.repo.Count(userID)
	if err != nil {
		return nil, err
	}
	draft := total - published
	return &Stats{
		Total: total, TotalPublished: published, TotalDraft: draft,
		TotalArticles: total, TotalSelesai: published, BelumSelesai: draft,
	}, nil
}

func (s *service) Create(userID uint, req PostRequest) (*PostResponse, error) {
	cat, err := normalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	slug, err := s.uniqueSlug(req.Title, 0)
	if err != nil {
		return nil, err
	}
	p := &Post{
		UserID: userID, Title: strings.TrimSpace(req.Title), Slug: slug, Category: cat,
		Excerpt: req.Excerpt, Content: req.Content, CoverImagePath: req.CoverImagePath,
	}
	s.setPublished(p, req.IsPublished)
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return s.MyDetail(userID, p.ID)
}

func (s *service) Update(userID, id uint, req PostRequest) (*PostResponse, error) {
	p, err := s.owned(userID, id)
	if err != nil {
		return nil, err
	}
	cat, err := normalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title != p.Title {
		slug, err := s.uniqueSlug(title, p.ID)
		if err != nil {
			return nil, err
		}
		p.Slug = slug
	}
	p.Title, p.Category, p.Excerpt, p.Content, p.CoverImagePath = title, cat, req.Excerpt, req.Content, req.CoverImagePath
	s.setPublished(p, req.IsPublished)
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	r := toResponse(*p)
	return &r, nil
}

func (s *service) TogglePublish(userID, id uint) (*PostResponse, error) {
	p, err := s.owned(userID, id)
	if err != nil {
		return nil, err
	}
	s.setPublished(p, !p.IsPublished)
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	r := toResponse(*p)
	return &r, nil
}

func (s *service) Delete(userID, id uint) error {
	p, err := s.owned(userID, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(p)
}

func (s *service) AdminUsers() ([]UserSummary, error) { return s.repo.ListUsers() }

func (s *service) AdminPosts() ([]PostResponse, error) {
	list, err := s.repo.ListAll()
	return toResponses(list), err
}

func (s *service) AdminUserPosts(userID uint) ([]PostResponse, error) {
	list, err := s.repo.ListByUser(userID)
	return toResponses(list), err
}
