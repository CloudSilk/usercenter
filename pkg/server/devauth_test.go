package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	pkgdb "github.com/CloudSilk/pkg/db"
	commonmodel "github.com/CloudSilk/pkg/model"
	"github.com/CloudSilk/usercenter/internal/apikeyauth"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	"github.com/CloudSilk/usercenter/internal/session"
	pb "github.com/CloudSilk/usercenter/proto"
	ucm "github.com/CloudSilk/usercenter/utils/middleware"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

func TestDevAuthRequiredAcceptsServiceAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(
		sqlite.Open("file:dev-auth-api-key?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	SetDB(pkgdb.NewDBClient(db, false))
	if err := db.AutoMigrate(&apikeyauth.APIKeyAuth{}); err != nil {
		t.Fatalf("migrate API keys: %v", err)
	}
	plaintext, err := apikeyauth.CreateKey(&apikeyauth.APIKeyAuth{
		TenantID:    "platform",
		PrincipalID: "runner-1",
		Name:        "Runner 1",
		Roles:       " ideasprint_runner, ",
	})
	if err != nil {
		t.Fatalf("create API key: %v", err)
	}

	router := gin.New()
	router.Use(DevAuthRequired)
	router.GET("/api/protected", func(c *gin.Context) {
		ok, user := ucm.GetUser(c)
		if !ok || user == nil {
			t.Fatal("service user missing from context")
		}
		c.JSON(http.StatusOK, gin.H{
			"tenantID": ucm.GetTenantID(c),
			"userID":   ucm.GetUserID(c),
			"roles":    user.RoleIDs,
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	request.Header.Set("X-API-Key", plaintext)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"protected status = %d, want %d; body=%s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}
	if got := response.Body.String(); got != `{"roles":["ideasprint_runner"],"tenantID":"platform","userID":"runner-1"}` {
		t.Fatalf("service context = %s", got)
	}
}

func TestDevAuthRequiredRejectsRevokedLoginSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	database, err := gorm.Open(
		sqlite.Open("file:dev-auth-revoked-session?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	SetDB(pkgdb.NewDBClient(database, false))
	if err := database.AutoMigrate(&session.Session{}); err != nil {
		t.Fatalf("migrate sessions: %v", err)
	}
	token.InitTokenCache("dev-auth-revoked-session-secret", "", "", "", 120)
	bearer, err := token.EncodeToken(&pb.CurrentUser{
		Id: "dev-auth-user", TenantID: "platform", UserName: "dev-auth-user", SessionID: "dev-auth-session",
	})
	if err != nil {
		t.Fatalf("encode token: %v", err)
	}
	loginSession := &session.Session{
		Model: commonmodel.Model{ID: "dev-auth-session"}, PrincipalID: "dev-auth-user", TenantID: "platform",
		TokenSig: token.GetTokenSignature(bearer), DeviceName: "Chrome · Windows",
	}
	if err := session.CreateSession(loginSession); err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := gin.New()
	router.Use(DevAuthRequired)
	router.GET("/api/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	request.Header.Set("Authorization", "Bearer "+bearer)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("active session status = %d, want %d; body=%s", response.Code, http.StatusNoContent, response.Body.String())
	}

	if err := session.RevokeSession(loginSession.ID, "test revoke"); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	request.Header.Set("Authorization", "Bearer "+bearer)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() == "" {
		t.Fatalf("revoked session status = %d, want token error; body=%s", response.Code, response.Body.String())
	}
}
