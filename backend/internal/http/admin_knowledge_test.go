package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenluuo/smart-project/backend/internal/knowledge"
	"github.com/gin-gonic/gin"
)

type adminKnowledgeServiceStub struct {
	items     []knowledge.AdminDocumentView
	total     int64
	listErr   error
	deleteErr error
	deletedID uint64
}

func (s *adminKnowledgeServiceStub) ListAll(_ context.Context, _ knowledge.AdminListFilter) ([]knowledge.AdminDocumentView, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.items, s.total, nil
}

func (s *adminKnowledgeServiceStub) Delete(_ context.Context, _ uint64, documentID uint64, _ string) (*knowledge.Document, error) {
	s.deletedID = documentID
	if s.deleteErr != nil {
		return nil, s.deleteErr
	}
	return &knowledge.Document{ID: documentID, Title: "已删文档", Category: "planting"}, nil
}

func newAdminKnowledgeRouter(auth authService, service adminKnowledgeService) *gin.Engine {
	router := NewRouter("test", pingerStub{}, auth)
	registerAdminKnowledgeRoutes(router, auth, service)
	return router
}

func TestAdminKnowledgeRequireSystemAdmin(t *testing.T) {
	router := newAdminKnowledgeRouter(authServiceStub{}, &adminKnowledgeServiceStub{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/docs", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/docs", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("farmer token: status = %d, want 403", response.Code)
	}
}

func TestAdminKnowledgeListAllStatuses(t *testing.T) {
	source := "农技站"
	service := &adminKnowledgeServiceStub{
		items: []knowledge.AdminDocumentView{{
			ID: 7, Title: "番茄种植手册", Category: "planting", Status: knowledge.StatusDraft,
			Version: 4, Source: &source, UploaderName: "codex08231008",
		}},
		total: 12,
	}
	router := newAdminKnowledgeRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/docs?status=DRAFT", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, want := range []string{`"code":0`, `"status":"DRAFT"`, `"uploaderName":"codex08231008"`, `"total":12`} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("body = %s, want it to contain %s", response.Body.String(), want)
		}
	}
}

func TestAdminKnowledgeDelete(t *testing.T) {
	service := &adminKnowledgeServiceStub{}
	router := newAdminKnowledgeRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/knowledge/docs/7", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || service.deletedID != 7 {
		t.Fatalf("status = %d, deletedID = %d, body = %s", response.Code, service.deletedID, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"indexCleanup":"queued"`) {
		t.Fatalf("body = %s, want indexCleanup queued", response.Body.String())
	}
}

func TestAdminKnowledgeDeleteNotFound(t *testing.T) {
	service := &adminKnowledgeServiceStub{deleteErr: knowledge.ErrNotFound}
	router := newAdminKnowledgeRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/knowledge/docs/999", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestAdminKnowledgeInvalidStatus(t *testing.T) {
	router := newAdminKnowledgeRouter(adminAuthServiceStub{}, &adminKnowledgeServiceStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/knowledge/docs?status=REJECTED", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}
