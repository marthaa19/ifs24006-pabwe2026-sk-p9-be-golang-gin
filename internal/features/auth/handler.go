package auth

import (
	"errors"
	"net/http"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler = controller endpoint /api/auth/*.
type Handler struct{ svc Service }

// NewHandler membuat Handler.
func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

// fromError memetakan kesalahan domain -> respons HTTP.
func fromError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDuplicate):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		response.Fail(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	default:
		response.Error(c, "Terjadi kesalahan pada server")
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data registrasi tidak valid (username min. 3, email valid, password min. 5)")
		return
	}
	res, err := h.svc.Register(req)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Registrasi berhasil", res)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Username dan password wajib diisi")
		return
	}
	res, err := h.svc.Login(req)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Login berhasil", res)
}

// Logout bersifat stateless: klien cukup membuang token-nya.
func (h *Handler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logout berhasil", nil)
}

func (h *Handler) Profile(c *gin.Context) {
	u, err := h.svc.Profile(middleware.CurrentUser(c).ID)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diambil", u)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data profil tidak valid")
		return
	}
	u, err := h.svc.UpdateProfile(middleware.CurrentUser(c).ID, req)
	if err != nil {
		fromError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diperbarui", u)
}
