package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newCtx() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) Body {
	var b Body
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	return b
}

func TestSuccess(t *testing.T) {
	c, w := newCtx()
	Success(c, 201, "ok", gin.H{"a": 1})
	assert.Equal(t, 201, w.Code)
	b := decode(t, w)
	assert.Equal(t, "success", b.Status)
	assert.NotNil(t, b.Data)
}

func TestFailTanpaData(t *testing.T) {
	c, w := newCtx()
	Fail(c, 400, "salah")
	assert.Equal(t, 400, w.Code)
	b := decode(t, w)
	assert.Equal(t, "fail", b.Status)
	assert.Nil(t, b.Data)
}

func TestError(t *testing.T) {
	c, w := newCtx()
	Error(c, "rusak")
	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "error", decode(t, w).Status)
}

func TestAbort(t *testing.T) {
	c, w := newCtx()
	AbortFail(c, 401, "x")
	assert.True(t, c.IsAborted())
	assert.Equal(t, 401, w.Code)

	c2, w2 := newCtx()
	AbortError(c2, "x")
	assert.True(t, c2.IsAborted())
	assert.Equal(t, 500, w2.Code)
}
