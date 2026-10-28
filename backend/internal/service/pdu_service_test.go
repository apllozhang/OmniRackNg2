package service

import (
    "errors"
    "testing"

    "omnirack-replica/backend/internal/model"
    "github.com/google/uuid"
)

func TestValidatePDUAllowsOptionalElectricalFields(t *testing.T) {
    rackID := uuid.New()
    err := validatePDU(model.PDU{RackID: rackID, Code: "PDU-01", Name: "机柜 PDU", Status: model.PDUStatusActive})
    if err != nil { t.Fatalf("expected optional electrical fields to be accepted, got %v", err) }
}

func TestValidatePDURejectsInvalidElectricalValues(t *testing.T) {
    value := -1.0
    err := validatePDU(model.PDU{RackID: uuid.New(), Code: "PDU-01", Name: "机柜 PDU", Status: model.PDUStatusActive, RatedPowerW: &value})
    if !errors.Is(err, ErrInvalidResource) { t.Fatalf("expected ErrInvalidResource, got %v", err) }
}

func TestValidateSocketSupportsCNAndEU10A16A(t *testing.T) {
    for _, standard := range []string{model.PDUSocketStandardCN, model.PDUSocketStandardEU} {
        for _, amperage := range []int{10, 16} {
            err := validateSocket(model.PDUSocket{PDUID: uuid.New(), SocketNo: 1, Standard: standard, AmperageA: amperage, Status: model.PDUSocketStatusAvailable})
            if err != nil { t.Fatalf("standard %s amperage %d: %v", standard, amperage, err) }
        }
    }
}

func TestValidateSocketRejectsUnsupportedStandardAndAmperage(t *testing.T) {
    if err := validateSocket(model.PDUSocket{PDUID: uuid.New(), SocketNo: 1, Standard: "US", AmperageA: 10}); !errors.Is(err, ErrInvalidResource) { t.Fatalf("expected invalid standard error, got %v", err) }
    if err := validateSocket(model.PDUSocket{PDUID: uuid.New(), SocketNo: 1, Standard: model.PDUSocketStandardCN, AmperageA: 20}); !errors.Is(err, ErrInvalidResource) { t.Fatalf("expected invalid amperage error, got %v", err) }
}

func TestNormalizePDUDefaultsAndTrims(t *testing.T) {
    pdu := model.PDU{Code: " pdu-1 ", Name: " PDU 1 ", Status: ""}
    normalizePDU(&pdu)
    if pdu.Code != "PDU-1" || pdu.Name != "PDU 1" || pdu.Status != model.PDUStatusActive { t.Fatalf("unexpected normalized PDU: %#v", pdu) }
}

func TestNormalizeSocketDefaults(t *testing.T) {
    socket := model.PDUSocket{PDUID: uuid.New(), Standard: " eu ", AmperageA: 16, Status: ""}
    normalizeSocket(&socket)
    if socket.Standard != model.PDUSocketStandardEU || socket.Status != model.PDUSocketStatusAvailable { t.Fatalf("unexpected normalized socket: %#v", socket) }
}