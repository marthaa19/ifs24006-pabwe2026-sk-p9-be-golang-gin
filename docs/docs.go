// Package docs menyajikan spesifikasi OpenAPI (Swagger 2.0) Delcom Blog API.
package docs

import (
	_ "embed"
	"encoding/json"
)

//go:embed swagger.json
var swaggerJSON []byte

// raw dapat diganti saat pengujian untuk mensimulasikan berkas rusak.
var raw = swaggerJSON

// GetSwaggerJSON mengembalikan spesifikasi dengan host & scheme sesuai pemanggil,
// sehingga tombol "Try it out" di Swagger UI selalu menembak server yang benar.
func GetSwaggerJSON(host, scheme string) ([]byte, error) {
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	spec["host"] = host
	spec["schemes"] = []string{scheme}
	return json.MarshalIndent(spec, "", "  ")
}
