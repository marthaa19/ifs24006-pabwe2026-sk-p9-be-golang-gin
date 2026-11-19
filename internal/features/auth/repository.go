package auth

import (
	"errors"

	"gorm.io/gorm"
)

// Repository mengisolasi seluruh query tabel users.
type Repository interface {
	Create(u *User) error
	FindByID(id uint) (*User, error)
	FindByLogin(login string) (*User, error)
	// Exists memeriksa apakah username/email sudah dipakai pengguna lain selain exceptID.
	Exists(username, email string, exceptID uint) (bool, error)
	Update(u *User) error
}

type gormRepository struct{ db *gorm.DB }

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository { return &gormRepository{db: db} }

func (r *gormRepository) Create(u *User) error { return r.db.Create(u).Error }

func (r *gormRepository) FindByID(id uint) (*User, error) {
	var u User
	if err := r.db.First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) FindByLogin(login string) (*User, error) {
	var u User
	if err := r.db.Where("username = ? OR email = ?", login, login).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *gormRepository) Exists(username, email string, exceptID uint) (bool, error) {
	var n int64
	err := r.db.Model(&User{}).
		Where("(username = ? OR email = ?) AND id <> ?", username, email, exceptID).
		Count(&n).Error
	return n > 0, err
}

func (r *gormRepository) Update(u *User) error { return r.db.Save(u).Error }
