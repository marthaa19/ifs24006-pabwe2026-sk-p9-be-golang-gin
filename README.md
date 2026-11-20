# Delcom Blog API (Golang Gin & PostgreSQL)

Backend REST API **Delcom Blog** - platform artikel multi-tier dengan AI Generator yang terintegrasi
Search Engine. Dibangun dengan Golang Gin, PostgreSQL 17, GORM, Clean Architecture (Feature-First), dan TDD.

## Menjalankan

```bash
psql -U postgres -c "CREATE DATABASE pabwe_p9_blog;"
cp .env.example .env        # sesuaikan DB_PASSWORD bila perlu
go mod tidy                 # mengunduh dependensi & membuat go.sum
go run .
```

- API aktif di `http://localhost:8081`
- Swagger UI: `http://localhost:8081/swagger/index.html`
- Akun bawaan: `admin / admin` (role admin) dan `member / member`
- Tanpa PostgreSQL? Set `DB_CONNECTION=sqlite` di `.env` (database memori, hilang saat server mati).

## Pengujian (TDD, 100% statement coverage)

```bash
go test -v -cover ./internal/...
```

## Build binary

```bash
go build -o server.exe .   # Windows
go build -o server .       # Linux/macOS
```

`.env` harus berada di folder yang sama saat biner dijalankan.
