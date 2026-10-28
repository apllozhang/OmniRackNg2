package handler

import (
    "net/http/httptest"
    "strings"
    "testing"

    "omnirack-replica/backend/internal/service"
    "github.com/gin-gonic/gin"
)

func TestPDUHandlerRejectsInvalidResourceID(t *testing.T) {
    gin.SetMode(gin.TestMode)
    h := NewPDUHandler(nil)
    req := httptest.NewRequest("GET", "/api/v1/pdus/not-a-uuid", nil)
    rec := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(rec)
    c.Request = req
    c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

    h.Get(c)
    if rec.Code != 400 { t.Fatalf("expected 400, got %d", rec.Code) }
    if !strings.Contains(rec.Body.String(), "INVALID_RESOURCE_ID") { t.Fatalf("expected INVALID_RESOURCE_ID response, got %s", rec.Body.String()) }
}

func TestWritePDUErrorMapsDomainErrors(t *testing.T) {
    gin.SetMode(gin.TestMode)
    rec := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(rec)
    writePDUError(c, service.ErrSocketConnected)
    if rec.Code != 409 || !strings.Contains(rec.Body.String(), "PDU_SOCKET_CONNECTED") { t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String()) }
}