package auth

import (
	"errors"
	"strings"
	"time"

	"app/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Kesalahan domain yang dipetakan handler menjadi kode HTTP.
var (
	ErrNotFound           = errors.New("pengguna tidak ditemukan")
	ErrDuplicate          = errors.New("username atau email sudah terdaftar")
	ErrInvalidCredentials = errors.New("username atau password salah")
)

// Service berisi logika bisnis autentikasi.
type Service interface {
	Register(req RegisterRequest) (*AuthResult, error)
	Login(req LoginRequest) (*AuthResult, error)
	Profile(id uint) (*User, error)
	UpdateProfile(id uint, req UpdateProfileRequest) (*User, error)
}

type service struct {
	repo   Repository
	secret string
	ttl    time.Duration
}

// NewService membuat Service dengan kunci JWT (APP_KEY) dan masa berlaku token.
func NewService(repo Repository, secret string, ttl time.Duration) Service {
	return &service{repo: repo, secret: secret, ttl: ttl}
}

// HashPassword membuat hash bcrypt dari password polos.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// signToken ditandatangani HS256; dijadikan variabel agar kegagalan bisa disimulasikan di test.
var signToken = func(claims middleware.Claims, secret string) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func (s *service) result(u *User) (*AuthResult, error) {
	t, err := signToken(middleware.Claims{
		UserID: u.ID, Username: u.Username, Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}, s.secret)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: t, User: u}, nil
}

func (s *service) Register(req RegisterRequest) (*AuthResult, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	taken, err := s.repo.Exists(username, email, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrDuplicate
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	role := RoleMember
	if strings.EqualFold(strings.TrimSpace(req.Role), RoleAdmin) {
		role = RoleAdmin
	}
	u := &User{Username: username, Email: email, Password: hash, Role: role}
	if err := s.repo.Create(u); err != nil {
		return nil, err
	}
	return s.result(u)
}

func (s *service) Login(req LoginRequest) (*AuthResult, error) {
	u, err := s.repo.FindByLogin(strings.TrimSpace(req.Username))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}
	return s.result(u)
}

func (s *service) Profile(id uint) (*User, error) { return s.repo.FindByID(id) }

func (s *service) UpdateProfile(id uint, req UpdateProfileRequest) (*User, error) {
	u, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = u.Username
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		email = u.Email
	}
	taken, err := s.repo.Exists(username, email, id)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrDuplicate
	}
	u.Username, u.Email = username, email
	if req.Password != "" {
		hash, err := HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		u.Password = hash
	}
	if err := s.repo.Update(u); err != nil {
		return nil, err
	}
	return u, nil
}
