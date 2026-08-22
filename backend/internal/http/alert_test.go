package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/alert"
)

type alertServiceStub struct {
	rules         []alert.RuleView
	updateResult  *alert.RuleUpdateResult
	listResult    alert.ListResult
	confirmResult *alert.ConfirmResult
	err           error
	ownerID       uint64
	plotID        uint64
	thresholdID   uint64
	alertID       uint64
	ruleInput     alert.RuleInput
	filter        alert.ListFilter
	remark        string
}

func (s *alertServiceStub) ListRules(_ context.Context, ownerID, plotID uint64) ([]alert.RuleView, error) {
	s.ownerID, s.plotID = ownerID, plotID
	return s.rules, s.err
}

func (s *alertServiceStub) UpsertRule(_ context.Context, ownerID, plotID, thresholdID uint64, input alert.RuleInput) (*alert.RuleUpdateResult, error) {
	s.ownerID, s.plotID, s.thresholdID, s.ruleInput = ownerID, plotID, thresholdID, input
	return s.updateResult, s.err
}

func (s *alertServiceStub) List(_ context.Context, ownerID uint64, filter alert.ListFilter) (alert.ListResult, error) {
	s.ownerID, s.filter = ownerID, filter
	return s.listResult, s.err
}

func (s *alertServiceStub) Confirm(_ context.Context, ownerID, alertID uint64, remark string) (*alert.ConfirmResult, error) {
	s.ownerID, s.alertID, s.remark = ownerID, alertID, remark
	return s.confirmResult, s.err
}

func newAlertTestRouter(service alertService) http.Handler {
	return NewRouterWithAllServices("test", pingerStub{}, authServiceStub{}, nil, nil, nil, service)
}

func TestAlertEndpointsRequireAuthentication(t *testing.T) {
	router := newAlertTestRouter(&alertServiceStub{})
	requests := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/plots/11/thresholds"},
		{http.MethodPut, "/api/v1/plots/11/thresholds/2"},
		{http.MethodGet, "/api/v1/alerts"},
		{http.MethodGet, "/api/v1/alerts/logs"},
		{http.MethodPost, "/api/v1/alerts/3/confirm"},
	}
	for _, requestData := range requests {
		request := httptest.NewRequest(requestData.method, requestData.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":40101`) {
			t.Fatalf("%s %s: status=%d body=%s", requestData.method, requestData.path, response.Code, response.Body.String())
		}
	}
}

func TestThresholdEndpointsFollowContract(t *testing.T) {
	now := time.Date(2026, 8, 22, 8, 20, 0, 0, time.UTC)
	service := &alertServiceStub{
		rules:        []alert.RuleView{{ID: 2, PlotID: 11, Metric: "soilMoisture", Operator: alert.OperatorLT, Value: 28, Unit: "%", DurationSeconds: 300, Enabled: true, Level: alert.LevelMedium}},
		updateResult: &alert.RuleUpdateResult{ID: 2, UpdatedAt: now},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/plots/11/thresholds", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.ownerID != 7 || service.plotID != 11 || !strings.Contains(response.Body.String(), `"operator":"LT"`) {
		t.Fatalf("GET thresholds: status=%d body=%s service=%+v", response.Code, response.Body.String(), service)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/plots/11/thresholds/2", strings.NewReader(`{"metric":"soilMoisture","operator":"LT","value":28,"durationSeconds":300,"level":"MEDIUM","enabled":false}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.thresholdID != 2 || service.ruleInput.Enabled || service.ruleInput.Value != 28 {
		t.Fatalf("PUT threshold: status=%d body=%s service=%+v", response.Code, response.Body.String(), service)
	}
}

func TestAlertListAndLogsParseFilters(t *testing.T) {
	service := &alertServiceStub{listResult: alert.ListResult{Items: []alert.ListItem{}, Page: 2, PageSize: 5, Total: 0}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/alerts?plotId=11&status=active&page=2&pageSize=5", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.filter.PlotID == nil || *service.filter.PlotID != 11 || service.filter.Status == nil || *service.filter.Status != alert.StatusActive {
		t.Fatalf("GET alerts: status=%d body=%s filter=%+v", response.Code, response.Body.String(), service.filter)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/alerts/logs?farmId=1&startTime=2026-08-15T00:00:00%2B08:00&endTime=2026-08-22T23:59:59%2B08:00", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response = httptest.NewRecorder()
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.filter.StartTime == nil || service.filter.EndTime == nil {
		t.Fatalf("GET alert logs: status=%d body=%s filter=%+v", response.Code, response.Body.String(), service.filter)
	}
}

func TestConfirmAlertAndErrors(t *testing.T) {
	now := time.Date(2026, 8, 22, 8, 23, 0, 0, time.UTC)
	service := &alertServiceStub{confirmResult: &alert.ConfirmResult{ID: 3, Status: alert.StatusConfirmed, ConfirmedAt: now}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/3/confirm", strings.NewReader(`{"remark":"已开启灌溉"}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.ownerID != 7 || service.alertID != 3 || service.remark != "已开启灌溉" || !strings.Contains(response.Body.String(), `"status":"CONFIRMED"`) {
		t.Fatalf("POST confirm: status=%d body=%s service=%+v", response.Code, response.Body.String(), service)
	}

	service.err = alert.ErrConflict
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/3/confirm", strings.NewReader(`{"remark":"重复确认"}`))
	request.Header.Set("Authorization", "Bearer signed-token")
	request.Header.Set("Content-Type", "application/json")
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":40903`) {
		t.Fatalf("conflict: status=%d body=%s", response.Code, response.Body.String())
	}

	service.err = errors.New("database unavailable")
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	newAlertTestRouter(service).ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("server error: status=%d body=%s", response.Code, response.Body.String())
	}
}
