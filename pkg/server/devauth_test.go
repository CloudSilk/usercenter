package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDevAuthRequiredAllowsLoginOptionsWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(DevAuthRequired)
	router.GET("/api/core/auth/login/options", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/core/auth/login/options", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("login options status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}
}

func TestDevAuthRequiredStillRejectsProtectedRequestsWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(DevAuthRequired)
	router.GET("/api/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("protected status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if response.Body.String() == "" {
		t.Fatal("protected response body is empty")
	}
}
