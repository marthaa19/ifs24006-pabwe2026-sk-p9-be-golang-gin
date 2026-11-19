// Package response menyeragamkan bentuk JSON seluruh endpoint: {status, message, data?}.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body adalah amplop standar balasan API.
type Body struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Success -> status "success" (HTTP 2xx).
func Success(c *gin.Context, code int, message string, data any) {
	c.JSON(code, Body{Status: "success", Message: message, Data: data})
}

// Fail -> status "fail" (HTTP 4xx, kesalahan dari klien).
func Fail(c *gin.Context, code int, message string) {
	c.JSON(code, Body{Status: "fail", Message: message})
}

// Error -> status "error" (HTTP 500, kesalahan di server).
func Error(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Body{Status: "error", Message: message})
}

// AbortFail sama seperti Fail tetapi menghentikan rantai middleware.
func AbortFail(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "fail", Message: message})
}

// AbortError sama seperti Error tetapi menghentikan rantai middleware.
func AbortError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, Body{Status: "error", Message: message})
}
