package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"app/internal/features/post"
)

// Kesalahan domain AI.
var (
	ErrTopicRequired = errors.New("topik artikel wajib diisi")
	ErrUnavailable   = errors.New("layanan AI sedang tidak tersedia")
	ErrBadResponse   = errors.New("jawaban AI tidak dapat dibaca")
)

// Service menyusun draf artikel dari topik (+ konteks mesin pencari).
type Service interface {
	Generate(req GenerateRequest) (*Draft, error)
}

type service struct {
	search Searcher
	chat   Completer
}

// NewService membuat Service.
func NewService(search Searcher, chat Completer) Service {
	return &service{search: search, chat: chat}
}

const systemPrompt = "Kamu adalah penulis blog berbahasa Indonesia. Balas HANYA dengan satu objek JSON " +
	`dengan kunci: "title", "category", "excerpt" (1-2 kalimat), dan "content" (artikel lengkap format Markdown, ` +
	"gunakan sub-judul ##). Kategori harus salah satu dari: " + "%s."

func buildMessages(topic, category string, sources []Source) []Message {
	var user strings.Builder
	fmt.Fprintf(&user, "Tulis artikel blog tentang: %s\nKategori yang diminta: %s\n", topic, category)
	if len(sources) > 0 {
		user.WriteString("\nGunakan fakta terkini dari hasil penelusuran berikut dan sebutkan rujukannya:\n")
		for i, s := range sources {
			fmt.Fprintf(&user, "%d. %s - %s (%s)\n", i+1, s.Title, s.Snippet, s.URL)
		}
	}
	return []Message{
		{Role: "system", Content: fmt.Sprintf(systemPrompt, strings.Join(post.Categories, ", "))},
		{Role: "user", Content: user.String()},
	}
}

// extractJSON mengambil objek JSON dari jawaban model (mengabaikan pagar ```json).
func extractJSON(raw string) (string, bool) {
	start, end := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return "", false
	}
	return raw[start : end+1], true
}

func (s *service) Generate(req GenerateRequest) (*Draft, error) {
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		return nil, ErrTopicRequired
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "Umum"
	}
	var sources []Source
	if req.IncludeWebSearch {
		// Penelusuran bersifat pelengkap: bila gagal, artikel tetap dibuat tanpa rujukan.
		if found, err := s.search.Search(topic); err == nil {
			sources = found
		}
	}
	raw, err := s.chat.Complete(buildMessages(topic, category, sources))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	blob, ok := extractJSON(raw)
	if !ok {
		return nil, ErrBadResponse
	}
	var d Draft
	if err := json.Unmarshal([]byte(blob), &d); err != nil || strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Content) == "" {
		return nil, ErrBadResponse
	}
	if !validCategory(d.Category) {
		d.Category = category
	}
	if !validCategory(d.Category) {
		d.Category = "Umum"
	}
	d.Sources = sources
	if len(sources) > 0 {
		d.Content += sourcesMarkdown(sources)
	}
	return &d, nil
}

func validCategory(c string) bool {
	for _, k := range post.Categories {
		if k == c {
			return true
		}
	}
	return false
}

func sourcesMarkdown(sources []Source) string {
	var b strings.Builder
	b.WriteString("\n\n## Sumber Rujukan\n")
	for _, s := range sources {
		fmt.Fprintf(&b, "- [%s](%s)\n", s.Title, s.URL)
	}
	return b.String()
}
