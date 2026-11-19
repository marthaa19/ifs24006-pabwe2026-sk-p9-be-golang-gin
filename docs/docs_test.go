package docs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSwaggerJSON(t *testing.T) {
	b, err := GetSwaggerJSON("blog.test:8081", "https")
	assert.NoError(t, err)
	var spec map[string]any
	assert.NoError(t, json.Unmarshal(b, &spec))
	assert.Equal(t, "blog.test:8081", spec["host"])
	assert.Equal(t, []any{"https"}, spec["schemes"])
	assert.Contains(t, spec["paths"], "/api/public/posts")
}

func TestGetSwaggerJSONRusak(t *testing.T) {
	old := raw
	defer func() { raw = old }()
	raw = []byte("{bukan json")
	_, err := GetSwaggerJSON("h", "http")
	assert.Error(t, err)
}
