package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

type adminUserServiceStub struct {
	items   []identity.AdminUserView
	total   int64
	listErr error
}

func (s *adminUserServiceStub) ListUsers(_ context.Context, _ identity.AdminUserFilter) ([]identity.AdminUserView, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.items, s.total, nil
}

func newAdminRouter(auth authService, users adminUserService) *gin.Engine {
	router := NewRouter("test", pingerStub{}, auth)
	registerAdminUserRoutes(router, auth, users)
	return router
}

func TestAdminUsersRequireSystemAdmin(t *testing.T) {
	router := newAdminRouter(authServiceStub{}, &adminUserServiceStub{})

	// 未登录 → 401
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", response.Code)
	}

	// FARMER → 403
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":40301`) {
		t.Fatalf("farmer token: status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAdminUsersList(t *testing.T) {
	createdAt := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	service := &adminUserServiceStub{
		items: []identity.AdminUserView{{
			ID: 2, Username: "farmer2", Mobile: "13800000002", Role: "FARMER",
			Status: identity.UserStatusActive, PlotCount: 3, CreatedAt: createdAt,
		}},
		total: 1,
	}
	router := newAdminRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?keyword=farmer", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, want := range []string{`"code":0`, `"username":"farmer2"`, `"role":"FARMER"`, `"plotCount":3`, `"total":1`} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("body = %s, want it to contain %s", response.Body.String(), want)
		}
	}
	if strings.Contains(response.Body.String(), "password") {
		t.Fatalf("response leaked password field: %s", response.Body.String())
	}
}

func TestAdminUsersInvalidStatus(t *testing.T) {
	router := newAdminRouter(adminAuthServiceStub{}, &adminUserServiceStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?status=UNKNOWN", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}
