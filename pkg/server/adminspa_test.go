package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterAdminSPAMountsEmbeddedManagementApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterAdminSPA(router)

	for _, path := range []string{"/web/admin", "/web/admin/users"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusOK)
		}
		if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
			t.Fatalf("%s content type = %q, want text/html", path, contentType)
		}
		if !strings.Contains(response.Body.String(), `<div id="root"></div>`) {
			t.Fatalf("%s did not serve the embedded admin application", path)
		}
	}
}
