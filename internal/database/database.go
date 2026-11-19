// Package database mengurus koneksi, migrasi, dan seeding.
package database

import (
	"app/internal/config"
	"app/internal/features/auth"
	"app/internal/features/post"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openPostgres dan openSQLite dijadikan variabel supaya bisa diganti saat pengujian.
var (
	openPostgres = func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	}
	openSQLite = func(dsn string) (*gorm.DB, error) {
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	}
)

// Connect membuka koneksi sesuai DB_CONNECTION (pgsql untuk runtime, sqlite untuk uji/lokal).
func Connect(cfg *config.Config) (*gorm.DB, error) {
	if cfg.DBConnection == "sqlite" {
		return OpenMemory()
	}
	return openPostgres(cfg.DSN())
}

// OpenMemory membuka SQLite in-memory (dipakai unit test & DB_CONNECTION=sqlite).
func OpenMemory() (*gorm.DB, error) {
	db, err := openSQLite(":memory:")
	if err != nil {
		return nil, err
	}
	// Satu koneksi saja: database memori hanya hidup selama koneksinya, jadi semua query harus berbagi koneksi itu.
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	db.Exec("PRAGMA foreign_keys = ON")
	return db, nil
}

// Migrate membuat/memperbarui tabel users dan posts.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&auth.User{}, &post.Post{})
}

// Seed membuat akun admin & member bawaan bila belum ada.
func Seed(db *gorm.DB, cfg *config.Config) error {
	accounts := []auth.User{
		{Username: cfg.AdminUsername, Email: cfg.AdminUsername + "@delcom.org", Role: auth.RoleAdmin},
		{Username: "member", Email: "member@delcom.org", Role: auth.RoleMember},
	}
	pass := map[string]string{cfg.AdminUsername: cfg.AdminPassword, "member": "member"}
	for _, a := range accounts {
		var n int64
		if err := db.Model(&auth.User{}).Where("username = ?", a.Username).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		hash, err := auth.HashPassword(pass[a.Username])
		if err != nil {
			return err
		}
		a.Password = hash
		if err := db.Create(&a).Error; err != nil {
			return err
		}
	}
	return nil
}

// Setup = Connect + Migrate + Seed dalam satu panggilan (dipakai main.go).
func Setup(cfg *config.Config) (*gorm.DB, error) {
	db, err := Connect(cfg)
	if err != nil {
		return nil, err
	}
	if err := Migrate(db); err != nil {
		return nil, err
	}
	if err := Seed(db, cfg); err != nil {
		return nil, err
	}
	return db, nil
}
