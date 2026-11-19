package ai

import (
	"errors"
	"net/http"

	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler = controller POST /api/posts/ai-generate.
type Handler struct{ svc Service }

// NewHandler membuat Handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// Generate menghasilkan draf artikel + daftar sumber rujukan.
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ErrTopicRequired.Error())
		return
	}
	draft, err := h.svc.Generate(req)
	switch {
	case err == nil:
		response.Success(c, http.StatusOK, "Draf artikel berhasil dibuat", draft)
	case errors.Is(err, ErrTopicRequired):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrUnavailable), errors.Is(err, ErrBadResponse):
		response.Fail(c, http.StatusBadGateway, err.Error())
	default:
		response.Error(c, "Terjadi kesalahan pada server")
	}
}
