package service

import (
	"errors"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrResourceNotFound = errors.New("resource not found")
	ErrDuplicateCode    = errors.New("duplicate resource code")
	ErrResourceVersion  = errors.New("resource version conflict")
	ErrParentDisabled   = errors.New("parent resource is disabled")
	ErrResourceLimit    = errors.New("resource limit exceeded")
	ErrHasChildren      = errors.New("resource has children")
	ErrInvalidResource  = errors.New("invalid resource")
)

// Actor identifies the user and request responsible for a write operation.
type Actor struct {
	UserID    *uuid.UUID
	RequestID string
	Source    string
}

// DataCenterCreateInput contains create-only options that are not persisted.
type DataCenterCreateInput struct {
	DataCenter       model.DataCenter
	AutoGenerateCode bool
}

// RoomWriteInput keeps transport-level field presence separate from the persisted room model.
// A nil RacksPerRow means the client omitted the field; AutoGenerateCode is honored only on create.
type RoomWriteInput struct {
	Room             model.Room
	RacksPerRow      *int
	AutoGenerateCode bool
}

// RackCreateInput contains create-only options that are not persisted.
type RackCreateInput struct {
	Rack                 model.Rack
	AutoGenerateCode     bool
	OverrideTemplateSpec bool
}

// ResourceCopyInput controls copy naming and the optional target parent.
type ResourceCopyInput struct {
	Version            uint
	Code               string
	Name               string
	AutoGenerateCode   bool
	TargetDataCenterID uuid.UUID
	TargetRoomID       uuid.UUID
}

// ResourceMoveInput controls a cross-parent move while preserving the resource identity.
type ResourceMoveInput struct {
	Version            uint
	Code               string
	AutoGenerateCode   bool
	TargetDataCenterID uuid.UUID
	TargetRoomID       uuid.UUID
}

// ResourceCopyResult reports the copied root and copied child counts.
type ResourceCopyResult struct {
	Resource    interface{} `json:"resource"`
	CopiedRooms int         `json:"copiedRooms"`
	CopiedRacks int         `json:"copiedRacks"`
}

// ResourceService implements data center, room and rack use cases.
type ResourceService struct {
	store          repository.ResourceStore
	templateReader repository.RackTemplateReader
}

func NewResourceService(store repository.ResourceStore, templateReaders ...repository.RackTemplateReader) *ResourceService {
	service := &ResourceService{store: store}
	if len(templateReaders) > 0 {
		service.templateReader = templateReaders[0]
	}
	return service
}
