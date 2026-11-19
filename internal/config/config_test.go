package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigDefaults(t *testing.T) {
	for _, k := range []string{"APP_PORT", "DB_HOST", "OPENAI_KEY"} {
		t.Setenv(k, "")
	}
	cfg := LoadConfig()
	assert.Equal(t, "8081", cfg.AppPort)
	assert.Equal(t, "127.0.0.1", cfg.DBHost)
	assert.Equal(t, "", cfg.OpenAIKey)
	assert.Equal(t, "admin", cfg.AdminUsername)
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DB_DATABASE", "uji")
	cfg := LoadConfig()
	assert.Equal(t, "9090", cfg.AppPort)
	assert.Equal(t, "uji", cfg.DBDatabase)
}

func TestLoadConfigReadsDotEnv(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(dir+"/.env", []byte("APP_KEY=dari_dotenv\n"), 0o600))
	old, _ := os.Getwd()
	assert.NoError(t, os.Chdir(dir))
	defer os.Chdir(old)
	os.Unsetenv("APP_KEY")
	cfg := LoadConfig()
	assert.Equal(t, "dari_dotenv", cfg.AppKey)
	os.Unsetenv("APP_KEY")
}

func TestDSN(t *testing.T) {
	cfg := &Config{DBHost: "h", DBPort: "1", DBUsername: "u", DBPassword: "p", DBDatabase: "d", DBSSLMode: "disable"}
	assert.Equal(t, "host=h port=1 user=u password=p dbname=d sslmode=disable TimeZone=Asia/Jakarta", cfg.DSN())
}
