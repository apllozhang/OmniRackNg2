package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type autoCodeDeviceStore struct {
	repository.DeviceStore
	deviceType model.DeviceType
	created    model.Device
}

func (s *autoCodeDeviceStore) GetDeviceType(context.Context, uuid.UUID) (model.DeviceType, error) {
	return s.deviceType, nil
}

func (s *autoCodeDeviceStore) WithinDeviceTransaction(_ context.Context, fn func(repository.DeviceStore) error) error {
	return fn(s)
}

func (s *autoCodeDeviceStore) CountDevices(context.Context) (int64, error) {
	return 0, nil
}

func (s *autoCodeDeviceStore) CreateDevice(_ context.Context, item *model.Device) error {
	s.created = *item
	return nil
}

func (s *autoCodeDeviceStore) CreateAuditLog(context.Context, *model.AuditLog) error {
	return nil
}

func TestCreateDeviceHandlerBindsAutoGenerateCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	typeID := uuid.New()
	store := &autoCodeDeviceStore{deviceType: model.DeviceType{
		BaseModel:      model.BaseModel{ID: typeID},
		Code:           "SERVER",
		Name:           "服务器",
		Category:       model.DeviceCategoryServer,
		Status:         model.DeviceTypeStatusActive,
		DefaultHeightU: 1,
	}}
	handler := NewDeviceHandler(service.NewDeviceService(store))
	router := gin.New()
	router.POST("/devices", handler.CreateDevice)

	body := []byte(`{"typeId":"` + typeID.String() + `","name":"自动编号设备","lifecycleStatus":"WAITING_RACK","heightU":1,"autoGenerateCode":true}`)
	request := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if matched := regexp.MustCompile(`^DEV-[0-9A-F]{12}$`).MatchString(store.created.Code); !matched {
		t.Fatalf("expected generated device code, got %q", store.created.Code)
	}

	var envelope struct {
		Code string       `json:"code"`
		Data model.Device `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != "SUCCESS" || envelope.Data.Code != store.created.Code {
		t.Fatalf("unexpected response: %+v", envelope)
	}
}
