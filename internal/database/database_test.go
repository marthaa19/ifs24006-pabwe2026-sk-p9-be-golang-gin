package database

import (
	"errors"
	"strings"
	"testing"

	"app/internal/config"
	"app/internal/features/auth"
	"app/internal/features/post"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func testCfg() *config.Config {
	return &config.Config{DBConnection: "sqlite", AdminUsername: "admin", AdminPassword: "admin"}
}

func TestOpenPostgresGagalTerhubung(t *testing.T) {
	_, err := openPostgres("host=127.0.0.1 port=1 user=x password=x dbname=x sslmode=disable connect_timeout=1")
	assert.Error(t, err)
}

func TestConnect(t *testing.T) {
	db, err := Connect(testCfg())
	assert.NoError(t, err)
	assert.NotNil(t, db)

	old := openPostgres
	defer func() { openPostgres = old }()
	called := ""
	openPostgres = func(dsn string) (*gorm.DB, error) { called = dsn; return nil, errors.New("down") }
	cfg := testCfg()
	cfg.DBConnection = "pgsql"
	cfg.DBHost = "db.local"
	_, err = Connect(cfg)
	assert.Error(t, err)
	assert.True(t, strings.Contains(called, "host=db.local"))
}

func TestOpenMemoryGagal(t *testing.T) {
	old := openSQLite
	defer func() { openSQLite = old }()
	openSQLite = func(string) (*gorm.DB, error) { return nil, errors.New("rusak") }
	_, err := OpenMemory()
	assert.Error(t, err)
}

func TestMigrateDanSeed(t *testing.T) {
	db, _ := OpenMemory()
	assert.NoError(t, Migrate(db))
	cfg := testCfg()
	assert.NoError(t, Seed(db, cfg))
	assert.NoError(t, Seed(db, cfg)) // idempoten: tidak membuat duplikat

	var users []auth.User
	db.Order("id").Find(&users)
	assert.Len(t, users, 2)
	assert.Equal(t, auth.RoleAdmin, users[0].Role)
	assert.Equal(t, "member", users[1].Username)
	assert.NotEqual(t, "admin", users[0].Password) // tersimpan sebagai hash

	// foreign key cascade aktif: hapus pengguna -> artikelnya ikut terhapus
	db.Create(&post.Post{UserID: users[1].ID, Title: "t", Slug: "t"})
	db.Delete(&users[1])
	var n int64
	db.Model(&post.Post{}).Count(&n)
	assert.Equal(t, int64(0), n)
}

func TestSeedKegagalan(t *testing.T) {
	// tabel belum ada
	db, _ := OpenMemory()
	assert.Error(t, Seed(db, testCfg()))

	// password terlalu panjang untuk bcrypt
	db, _ = OpenMemory()
	assert.NoError(t, Migrate(db))
	cfg := testCfg()
	cfg.AdminPassword = strings.Repeat("x", 80)
	assert.Error(t, Seed(db, cfg))

	// insert ditolak oleh trigger
	db, _ = OpenMemory()
	assert.NoError(t, Migrate(db))
	assert.NoError(t, db.Exec("CREATE TRIGGER tolak BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'ditolak'); END;").Error)
	assert.Error(t, Seed(db, testCfg()))
}

func TestSetup(t *testing.T) {
	db, err := Setup(testCfg())
	assert.NoError(t, err)
	var n int64
	db.Model(&auth.User{}).Count(&n)
	assert.Equal(t, int64(2), n)

	// koneksi gagal
	old := openPostgres
	defer func() { openPostgres = old }()
	openPostgres = func(string) (*gorm.DB, error) { return nil, errors.New("down") }
	cfg := testCfg()
	cfg.DBConnection = "pgsql"
	_, err = Setup(cfg)
	assert.Error(t, err)

	// migrasi gagal: koneksi sudah ditutup
	oldS := openSQLite
	defer func() { openSQLite = oldS }()
	openSQLite = func(dsn string) (*gorm.DB, error) {
		d, e := oldS(dsn)
		s, _ := d.DB()
		_ = s.Close()
		return d, e
	}
	_, err = Setup(testCfg())
	assert.Error(t, err)
	openSQLite = oldS

	// seed gagal
	bad := testCfg()
	bad.AdminPassword = strings.Repeat("x", 80)
	_, err = Setup(bad)
	assert.Error(t, err)
}
