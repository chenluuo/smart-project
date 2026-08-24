package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminPlotServiceStub struct {
	items         []plot.AdminPlotItem
	total         int64
	listErr       error
	createErr     error
	assignErr     error
	createdInput  plot.CreateInput
	assignedOwner uint64
}

func (s *adminPlotServiceStub) AdminList(_ context.Context, _ plot.AdminListFilter) ([]plot.AdminPlotItem, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.items, s.total, nil
}

func (s *adminPlotServiceStub) CreatePlot(_ context.Context, input plot.CreateInput) (*plot.Plot, error) {
	s.createdInput = input
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &plot.Plot{ID: 21, OwnerID: input.OwnerID, Code: input.Code, Name: input.Name, Status: plot.StatusActive}, nil
}

func (s *adminPlotServiceStub) AssignOwner(_ context.Context, plotID, ownerID uint64) (*plot.Plot, error) {
	s.assignedOwner = ownerID
	if s.assignErr != nil {
		return nil, s.assignErr
	}
	return &plot.Plot{ID: plotID, OwnerID: ownerID, Code: "A3", Name: "A3 地块", Status: plot.StatusActive}, nil
}

func newAdminPlotRouter(auth authService, service adminPlotService) *gin.Engine {
	router := NewRouter("test", pingerStub{}, auth)
	registerAdminPlotRoutes(router, auth, service)
	return router
}

func TestAdminPlotsRequireSystemAdmin(t *testing.T) {
	router := newAdminPlotRouter(authServiceStub{}, &adminPlotServiceStub{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/plots", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/plots", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("farmer token: status = %d, want 403", response.Code)
	}
}

func TestAdminPlotsList(t *testing.T) {
	ownerName := "farmer2"
	service := &adminPlotServiceStub{
		items: []plot.AdminPlotItem{{
			Plot: plot.Plot{ID: 3, OwnerID: 2, Code: "A3", Name: "A3 番茄试验田", Status: plot.StatusActive},
			OwnerName: &ownerName, DeviceCount: 2,
		}},
		total: 1,
	}
	router := newAdminPlotRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/plots", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, want := range []string{`"code":0`, `"id":3`, `"ownerId":2`, `"ownerName":"farmer2"`, `"deviceCount":2`, `"total":1`} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("body = %s, want it to contain %s", response.Body.String(), want)
		}
	}
}

func TestAdminPlotsCreate(t *testing.T) {
	service := &adminPlotServiceStub{}
	router := newAdminPlotRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/plots", strings.NewReader(`{"code":"B2","name":"B2 西瓜田","area":8.6,"ownerId":2}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.createdInput.Code != "B2" || service.createdInput.OwnerID != 2 {
		t.Fatalf("createdInput = %+v, want code B2 owner 2", service.createdInput)
	}
	if !strings.Contains(response.Body.String(), `"id":21`) {
		t.Fatalf("body = %s, want created plot id", response.Body.String())
	}
}

func TestAdminPlotsCreateInvalid(t *testing.T) {
	router := newAdminPlotRouter(adminAuthServiceStub{}, &adminPlotServiceStub{})
	for _, body := range []string{
		`{"code":"","name":"B2","ownerId":2}`,
		`{"code":"B2","name":"","ownerId":2}`,
		`{"code":"B2","name":"B2"}`,
		`{"code":"B2","name":"B2","ownerId":0}`,
		`not-json`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/plots", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer signed-token")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %q: status = %d, want 400", body, response.Code)
		}
	}
}

func TestAdminPlotsCreateConflict(t *testing.T) {
	service := &adminPlotServiceStub{createErr: gorm.ErrDuplicatedKey}
	router := newAdminPlotRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/plots", strings.NewReader(`{"code":"A3","name":"重复","ownerId":2}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAdminPlotsAssignOwner(t *testing.T) {
	service := &adminPlotServiceStub{}
	router := newAdminPlotRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/plots/3/owner", strings.NewReader(`{"ownerId":4}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || service.assignedOwner != 4 {
		t.Fatalf("status = %d, assignedOwner = %d, body = %s", response.Code, service.assignedOwner, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"ownerId":4`) {
		t.Fatalf("body = %s, want ownerId 4", response.Body.String())
	}
}

func TestAdminPlotsAssignOwnerNotFound(t *testing.T) {
	service := &adminPlotServiceStub{assignErr: gorm.ErrRecordNotFound}
	router := newAdminPlotRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/plots/999/owner", strings.NewReader(`{"ownerId":4}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
