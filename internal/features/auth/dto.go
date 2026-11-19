package auth

import "time"

// User adalah model GORM tabel users.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:255;uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	Role      string    `gorm:"size:50;not null;default:member" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Role yang dikenal sistem.
const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// RegisterRequest = payload POST /api/auth/register.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=5"`
}

// LoginRequest = payload POST /api/auth/login (login boleh memakai username atau email).
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UpdateProfileRequest = payload PUT /api/auth/profile (password opsional).
type UpdateProfileRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"omitempty,min=5"`
}

// AuthResult dikembalikan setelah register/login berhasil.
type AuthResult struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}
