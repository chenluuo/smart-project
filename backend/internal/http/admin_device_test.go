package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenluuo/smart-project/backend/internal/device"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminDeviceServiceStub struct {
	items       []device.AdminDeviceItem
	total       int64
	listErr     error
	bindErr     error
	unbindErr   error
	boundInput  device.BindInput
	unboundID   uint64
}

func (s *adminDeviceServiceStub) AdminList(_ context.Context, _ device.ListFilter) ([]device.AdminDeviceItem, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.items, s.total, nil
}

func (s *adminDeviceServiceStub) AdminBind(_ context.Context, _ uint64, input device.BindInput) (*device.Device, error) {
	s.boundInput = input
	if s.bindErr != nil {
		return nil, s.bindErr
	}
	return &device.Device{ID: 7, SerialNo: input.SerialNo, Name: input.Name, DeviceType: input.DeviceType, Status: device.StatusOffline}, nil
}

func (s *adminDeviceServiceStub) AdminUnbind(_ context.Context, deviceID uint64) error {
	s.unboundID = deviceID
	return s.unbindErr
}

func newAdminDeviceRouter(auth authService, service adminDeviceService) *gin.Engine {
	router := NewRouter("test", pingerStub{}, auth)
	registerAdminDeviceRoutes(router, auth, service)
	return router
}

func TestAdminDevicesRequireSystemAdmin(t *testing.T) {
	router := newAdminDeviceRouter(authServiceStub{}, &adminDeviceServiceStub{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/devices", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", response.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/devices", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("farmer token: status = %d, want 403", response.Code)
	}
}

func TestAdminDevicesList(t *testing.T) {
	plotCode := "A1"
	plotName := "A1 番茄地"
	ownerName := "testfarmer"
	service := &adminDeviceServiceStub{
		items: []device.AdminDeviceItem{{
			Device: device.Device{ID: 1, SerialNo: "SN-001", Name: "A1 土壤传感器", DeviceType: "SOIL_SENSOR", Status: device.StatusOnline},
			PlotID: 1, PlotCode: &plotCode, PlotName: &plotName, OwnerName: &ownerName,
		}},
		total: 1,
	}
	router := newAdminDeviceRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/devices", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	for _, want := range []string{`"code":0`, `"deviceSn":"SN-001"`, `"plotId":1`, `"plotName":"A1 番茄地"`, `"ownerName":"testfarmer"`, `"total":1`} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("body = %s, want it to contain %s", response.Body.String(), want)
		}
	}
}

func TestAdminDevicesBindToAnyPlot(t *testing.T) {
	service := &adminDeviceServiceStub{}
	router := newAdminDeviceRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/devices/bind",
		strings.NewReader(`{"deviceSn":"SN-X","plotId":1,"name":"新设备","type":"VALVE"}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.boundInput.PlotID != 1 || service.boundInput.SerialNo != "SN-X" {
		t.Fatalf("boundInput = %+v, want plot 1 / SN-X", service.boundInput)
	}
}

func TestAdminDevicesUnbind(t *testing.T) {
	service := &adminDeviceServiceStub{}
	router := newAdminDeviceRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/devices/3/binding", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || service.unboundID != 3 {
		t.Fatalf("status = %d, unboundID = %d, body = %s", response.Code, service.unboundID, response.Body.String())
	}
}

func TestAdminDevicesUnbindNotFound(t *testing.T) {
	service := &adminDeviceServiceStub{unbindErr: gorm.ErrRecordNotFound}
	router := newAdminDeviceRouter(adminAuthServiceStub{}, service)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/devices/999/binding", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
