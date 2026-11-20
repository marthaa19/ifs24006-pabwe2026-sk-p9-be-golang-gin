package post

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler = controller endpoint /api/public/*, /api/posts/*, /api/admin/*.
type Handler struct{ svc Service }

// NewHandler membuat Handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func fromError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrForbidden):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrInvalidCategory):
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
	default:
		response.Error(c, "Terjadi kesalahan pada server")
	}
}

// paramID membaca :id dari URL; balasan 400 dikirim sendiri bila tidak valid.
func paramID(c *gin.Context) (uint, bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID tidak valid")
		return 0, false
	}
	return uint(n), true
}

func uid(c *gin.Context) uint { return middleware.CurrentUser(c).ID }

// ---------- publik ----------

func (h *Handler) Categories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Daftar kategori berhasil diambil", h.svc.Categories())
}

func (h *Handler) PublicList(c *gin.Context) {
	list, err := h.svc.PublicList(c.Query("q"), c.Query("category"))
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel berhasil diambil", list)
}

func (h *Handler) PublicDetail(c *gin.Context) {
	p, err := h.svc.PublicDetail(c.Param("slug"))
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", p)
}

// ---------- penulis ----------

func (h *Handler) MyList(c *gin.Context) {
	list, err := h.svc.MyList(uid(c))
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel Anda berhasil diambil", list)
}

func (h *Handler) Stats(c *gin.Context) {
	st, err := h.svc.Stats(uid(c))
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Statistik berhasil diambil", st)
}

func (h *Handler) MyDetail(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	p, err := h.svc.MyDetail(uid(c), id)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diambil", p)
}

func (h *Handler) Create(c *gin.Context) {
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Judul dan konten artikel wajib diisi")
		return
	}
	p, err := h.svc.Create(uid(c), req)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Artikel berhasil dibuat", p)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	cur, err := h.svc.MyDetail(uid(c), id) // 404 bila tidak ada, 403 bila milik orang lain
	if err != nil {
		fromError(c, err)
		return
	}
	var in UpdateRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data artikel tidak valid")
		return
	}
	req := mergeUpdate(cur, in)
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		response.Fail(c, http.StatusBadRequest, "Judul dan konten artikel wajib diisi")
		return
	}
	p, err := h.svc.Update(uid(c), id, req)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diperbarui", p)
}

// mergeUpdate menimpa artikel saat ini hanya dengan kolom yang dikirim klien.
func mergeUpdate(cur *PostResponse, in UpdateRequest) PostRequest {
	req := PostRequest{}
	if cur != nil {
		req = PostRequest{
			Title: cur.Title, Category: cur.Category, Excerpt: cur.Excerpt, Content: cur.Content,
			CoverImagePath: cur.CoverImagePath, IsPublished: cur.IsPublished,
		}
	}
	if in.Title != nil {
		req.Title = *in.Title
	}
	if in.Category != nil {
		req.Category = *in.Category
	}
	if in.Excerpt != nil {
		req.Excerpt = *in.Excerpt
	}
	if in.Content != nil {
		req.Content = *in.Content
	}
	if in.CoverImagePath != nil {
		req.CoverImagePath = *in.CoverImagePath
	}
	if in.IsPublished != nil {
		req.IsPublished = *in.IsPublished
	}
	return req
}

func (h *Handler) Toggle(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	p, err := h.svc.TogglePublish(uid(c), id)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Status publikasi berhasil diubah", p)
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(uid(c), id); err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil dihapus", nil)
}

// ---------- admin ----------

func (h *Handler) AdminUsers(c *gin.Context) {
	list, err := h.svc.AdminUsers()
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar pengguna berhasil diambil", list)
}

func (h *Handler) AdminPosts(c *gin.Context) {
	list, err := h.svc.AdminPosts()
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Seluruh artikel berhasil diambil", list)
}

func (h *Handler) AdminUserPosts(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	list, err := h.svc.AdminUserPosts(id)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel pengguna berhasil diambil", list)
}
