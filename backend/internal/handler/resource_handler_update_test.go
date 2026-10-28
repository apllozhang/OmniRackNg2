package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeResourceService struct {
	createDataCenter func(context.Context, service.Actor, service.DataCenterCreateInput) (model.DataCenter, error)
	createRoom       func(context.Context, service.Actor, uuid.UUID, service.RoomWriteInput) (model.Room, error)
	createRack       func(context.Context, service.Actor, uuid.UUID, service.RackCreateInput) (model.Rack, error)
	updateDataCenter func(context.Context, service.Actor, uuid.UUID, uint, model.DataCenter) (model.DataCenter, error)
	updateRoom       func(context.Context, service.Actor, uuid.UUID, uint, service.RoomWriteInput) (model.Room, error)
	updateRack       func(context.Context, service.Actor, uuid.UUID, uint, model.Rack) (model.Rack, error)
	copyDataCenter   func(context.Context, service.Actor, uuid.UUID, service.ResourceCopyInput) (service.ResourceCopyResult, error)
	copyRoom         func(context.Context, service.Actor, uuid.UUID, service.ResourceCopyInput) (service.ResourceCopyResult, error)
	copyRack         func(context.Context, service.Actor, uuid.UUID, service.ResourceCopyInput) (service.ResourceCopyResult, error)
	moveRoom         func(context.Context, service.Actor, uuid.UUID, service.ResourceMoveInput) (model.Room, error)
	moveRack         func(context.Context, service.Actor, uuid.UUID, service.ResourceMoveInput) (model.Rack, error)
}

func (f *fakeResourceService) ListTree(context.Context) ([]model.DataCenter, error) { return nil, nil }
func (f *fakeResourceService) CopyDataCenter(ctx context.Context, actor service.Actor, id uuid.UUID, input service.ResourceCopyInput) (service.ResourceCopyResult, error) {
	if f.copyDataCenter == nil {
		return service.ResourceCopyResult{}, errors.New("unexpected CopyDataCenter call")
	}
	return f.copyDataCenter(ctx, actor, id, input)
}
func (f *fakeResourceService) CopyRoom(ctx context.Context, actor service.Actor, id uuid.UUID, input service.ResourceCopyInput) (service.ResourceCopyResult, error) {
	if f.copyRoom == nil {
		return service.ResourceCopyResult{}, errors.New("unexpected CopyRoom call")
	}
	return f.copyRoom(ctx, actor, id, input)
}
func (f *fakeResourceService) CopyRack(ctx context.Context, actor service.Actor, id uuid.UUID, input service.ResourceCopyInput) (service.ResourceCopyResult, error) {
	if f.copyRack == nil {
		return service.ResourceCopyResult{}, errors.New("unexpected CopyRack call")
	}
	return f.copyRack(ctx, actor, id, input)
}
func (f *fakeResourceService) MoveRoom(ctx context.Context, actor service.Actor, id uuid.UUID, input service.ResourceMoveInput) (model.Room, error) {
	if f.moveRoom == nil {
		return model.Room{}, errors.New("unexpected MoveRoom call")
	}
	return f.moveRoom(ctx, actor, id, input)
}
func (f *fakeResourceService) MoveRack(ctx context.Context, actor service.Actor, id uuid.UUID, input service.ResourceMoveInput) (model.Rack, error) {
	if f.moveRack == nil {
		return model.Rack{}, errors.New("unexpected MoveRack call")
	}
	return f.moveRack(ctx, actor, id, input)
}

func (f *fakeResourceService) CreateDataCenter(ctx context.Context, actor service.Actor, input service.DataCenterCreateInput) (model.DataCenter, error) {
	if f.createDataCenter == nil {
		return model.DataCenter{}, nil
	}
	return f.createDataCenter(ctx, actor, input)
}
func (f *fakeResourceService) UpdateDataCenter(ctx context.Context, actor service.Actor, id uuid.UUID, version uint, item model.DataCenter) (model.DataCenter, error) {
	if f.updateDataCenter == nil {
		return model.DataCenter{}, errors.New("unexpected UpdateDataCenter call")
	}
	return f.updateDataCenter(ctx, actor, id, version, item)
}
func (f *fakeResourceService) DeleteDataCenter(context.Context, service.Actor, uuid.UUID, uint) error {
	return nil
}
func (f *fakeResourceService) CreateRoom(ctx context.Context, actor service.Actor, parentID uuid.UUID, input service.RoomWriteInput) (model.Room, error) {
	if f.createRoom == nil {
		return model.Room{}, nil
	}
	return f.createRoom(ctx, actor, parentID, input)
}
func (f *fakeResourceService) UpdateRoom(ctx context.Context, actor service.Actor, id uuid.UUID, version uint, input service.RoomWriteInput) (model.Room, error) {
	if f.updateRoom == nil {
		return model.Room{}, errors.New("unexpected UpdateRoom call")
	}
	return f.updateRoom(ctx, actor, id, version, input)
}
func (f *fakeResourceService) DeleteRoom(context.Context, service.Actor, uuid.UUID, uint) error {
	return nil
}
func (f *fakeResourceService) CreateRack(ctx context.Context, actor service.Actor, parentID uuid.UUID, input service.RackCreateInput) (model.Rack, error) {
	if f.createRack == nil {
		return model.Rack{}, nil
	}
	return f.createRack(ctx, actor, parentID, input)
}
func (f *fakeResourceService) UpdateRack(ctx context.Context, actor service.Actor, id uuid.UUID, version uint, item model.Rack) (model.Rack, error) {
	if f.updateRack == nil {
		return model.Rack{}, errors.New("unexpected UpdateRack call")
	}
	return f.updateRack(ctx, actor, id, version, item)
}
func (f *fakeResourceService) DeleteRack(context.Context, service.Actor, uuid.UUID, uint) error {
	return nil
}

type handlerEnvelope struct {
	Code      string          `json:"code"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

func performResourceUpdate(t *testing.T, handler *ResourceHandler, route, target, body string, register func(*gin.Engine, *ResourceHandler)) (*httptest.ResponseRecorder, handlerEnvelope) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	actorID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	router.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, actorID.String())
		c.Set("requestId", "req-handler-update")
		c.Next()
	})
	register(router, handler)

	request := httptest.NewRequest(http.MethodPut, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var envelope handlerEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %s response: %v; body=%s", route, err, recorder.Body.String())
	}
	return recorder, envelope
}

func performResourceCreate(t *testing.T, handler *ResourceHandler, target, body string, register func(*gin.Engine, *ResourceHandler)) (*httptest.ResponseRecorder, handlerEnvelope) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("requestId", "req-handler-update")
		c.Next()
	})
	register(router, handler)
	request := httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var envelope handlerEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode create response: %v; body=%s", err, recorder.Body.String())
	}
	return recorder, envelope
}

func TestCreateResourceHandlersPassAutoGenerateCode(t *testing.T) {
	dataCenterFake := &fakeResourceService{createDataCenter: func(_ context.Context, _ service.Actor, input service.DataCenterCreateInput) (model.DataCenter, error) {
		if !input.AutoGenerateCode || input.DataCenter.Name != "自动中心" || input.DataCenter.Code != "" {
			t.Fatalf("unexpected data center create input: %+v", input)
		}
		return model.DataCenter{Code: "DC-TEST", Name: input.DataCenter.Name}, nil
	}}
	withResourceHandler(t, dataCenterFake, func(handler *ResourceHandler) {
		recorder, envelope := performResourceCreate(t, handler, "/data-centers", `{"name":"自动中心","autoGenerateCode":true}`, func(router *gin.Engine, handler *ResourceHandler) {
			router.POST("/data-centers", handler.CreateDataCenter)
		})
		assertHandlerResponse(t, recorder, envelope, http.StatusCreated, "SUCCESS")
	})

	dataCenterID := uuid.New()
	roomFake := &fakeResourceService{createRoom: func(_ context.Context, _ service.Actor, gotID uuid.UUID, input service.RoomWriteInput) (model.Room, error) {
		if gotID != dataCenterID || !input.AutoGenerateCode || input.Room.Name != "自动机房" {
			t.Fatalf("unexpected room create input: parent=%s input=%+v", gotID, input)
		}
		return model.Room{Code: "ROOM-TEST", Name: input.Room.Name}, nil
	}}
	withResourceHandler(t, roomFake, func(handler *ResourceHandler) {
		recorder, envelope := performResourceCreate(t, handler, "/data-centers/"+dataCenterID.String()+"/rooms", `{"name":"自动机房","autoGenerateCode":true,"racksPerRow":12}`, func(router *gin.Engine, handler *ResourceHandler) {
			router.POST("/data-centers/:dataCenterId/rooms", handler.CreateRoom)
		})
		assertHandlerResponse(t, recorder, envelope, http.StatusCreated, "SUCCESS")
	})

	roomID := uuid.New()
	rackFake := &fakeResourceService{createRack: func(_ context.Context, _ service.Actor, gotID uuid.UUID, input service.RackCreateInput) (model.Rack, error) {
		if gotID != roomID || !input.AutoGenerateCode || !input.OverrideTemplateSpec || input.Rack.Name != "自动机柜" || input.Rack.WidthMm != 900 {
			t.Fatalf("unexpected rack create input: parent=%s input=%+v", gotID, input)
		}
		return model.Rack{Code: "RACK-TEST", Name: input.Rack.Name}, nil
	}}
	withResourceHandler(t, rackFake, func(handler *ResourceHandler) {
		recorder, envelope := performResourceCreate(t, handler, "/rooms/"+roomID.String()+"/racks", `{"name":"自动机柜","autoGenerateCode":true,"overrideTemplateSpec":true,"widthMm":900}`, func(router *gin.Engine, handler *ResourceHandler) {
			router.POST("/rooms/:roomId/racks", handler.CreateRack)
		})
		assertHandlerResponse(t, recorder, envelope, http.StatusCreated, "SUCCESS")
	})
}

func withResourceHandler(t *testing.T, fake *fakeResourceService, fn func(*ResourceHandler)) {
	t.Helper()
	fn(NewResourceHandler(fake))
}

func assertHandlerResponse(t *testing.T, recorder *httptest.ResponseRecorder, envelope handlerEnvelope, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, status, recorder.Body.String())
	}
	if envelope.Code != code {
		t.Fatalf("code = %q, want %q; body=%s", envelope.Code, code, recorder.Body.String())
	}
	if envelope.RequestID != "req-handler-update" {
		t.Fatalf("requestId = %q, want req-handler-update", envelope.RequestID)
	}
}

func TestUpdateDataCenterHandler(t *testing.T) {
	id := uuid.New()
	register := func(router *gin.Engine, handler *ResourceHandler) {
		router.PUT("/data-centers/:id", handler.UpdateDataCenter)
	}

	t.Run("success passes id version actor and payload", func(t *testing.T) {
		called := false
		fake := &fakeResourceService{updateDataCenter: func(_ context.Context, actor service.Actor, gotID uuid.UUID, version uint, item model.DataCenter) (model.DataCenter, error) {
			called = true
			if gotID != id || version != 3 || item.Code != "dc-01" {
				t.Fatalf("unexpected service arguments: id=%s version=%d item=%+v", gotID, version, item)
			}
			if actor.UserID == nil || actor.RequestID != "req-handler-update" || actor.Source != "web" {
				t.Fatalf("unexpected actor: %+v", actor)
			}
			item.ID = gotID
			item.Version = version + 1
			return item, nil
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "data center", "/data-centers/"+id.String(), `{"code":"dc-01","name":"中心","version":3}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusOK, "SUCCESS")
		})
		if !called {
			t.Fatal("service was not called")
		}
	})

	t.Run("invalid id is rejected before service", func(t *testing.T) {
		withResourceHandler(t, &fakeResourceService{}, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "data center", "/data-centers/not-a-uuid", `{}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE_ID")
		})
	})

	t.Run("invalid json is rejected", func(t *testing.T) {
		withResourceHandler(t, &fakeResourceService{}, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "data center", "/data-centers/"+id.String(), `{`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_REQUEST")
		})
	})

	t.Run("zero version is surfaced as invalid resource", func(t *testing.T) {
		fake := &fakeResourceService{updateDataCenter: func(_ context.Context, _ service.Actor, _ uuid.UUID, version uint, _ model.DataCenter) (model.DataCenter, error) {
			if version != 0 {
				t.Fatalf("version = %d, want 0", version)
			}
			return model.DataCenter{}, service.ErrInvalidResource
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "data center", "/data-centers/"+id.String(), `{"code":"DC-01","name":"中心","version":0}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE")
		})
	})

	t.Run("version conflict returns 409", func(t *testing.T) {
		fake := &fakeResourceService{updateDataCenter: func(context.Context, service.Actor, uuid.UUID, uint, model.DataCenter) (model.DataCenter, error) {
			return model.DataCenter{}, service.ErrResourceVersion
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "data center", "/data-centers/"+id.String(), `{"code":"DC-01","name":"中心","version":3}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusConflict, "RESOURCE_VERSION_CONFLICT")
		})
	})
}

func TestUpdateRoomHandler(t *testing.T) {
	id := uuid.New()
	register := func(router *gin.Engine, handler *ResourceHandler) {
		router.PUT("/rooms/:id", handler.UpdateRoom)
	}

	t.Run("omitted racksPerRow remains nil", func(t *testing.T) {
		fake := &fakeResourceService{updateRoom: func(_ context.Context, _ service.Actor, gotID uuid.UUID, version uint, input service.RoomWriteInput) (model.Room, error) {
			if gotID != id || version != 5 || input.RacksPerRow != nil {
				t.Fatalf("unexpected service arguments: id=%s version=%d racksPerRow=%v", gotID, version, input.RacksPerRow)
			}
			input.Room.ID = gotID
			input.Room.Version = version + 1
			return input.Room, nil
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "room", "/rooms/"+id.String(), `{"code":"ROOM-01","name":"机房","version":5}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusOK, "SUCCESS")
		})
	})

	t.Run("explicit zero reaches service and returns 400", func(t *testing.T) {
		fake := &fakeResourceService{updateRoom: func(_ context.Context, _ service.Actor, _ uuid.UUID, _ uint, input service.RoomWriteInput) (model.Room, error) {
			if input.RacksPerRow == nil || *input.RacksPerRow != 0 {
				t.Fatalf("explicit zero was not preserved: %v", input.RacksPerRow)
			}
			return model.Room{}, service.ErrInvalidResource
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "room", "/rooms/"+id.String(), `{"code":"ROOM-01","name":"机房","version":5,"racksPerRow":0}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE")
		})
	})

	t.Run("invalid id is rejected", func(t *testing.T) {
		withResourceHandler(t, &fakeResourceService{}, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "room", "/rooms/bad-id", `{}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE_ID")
		})
	})

	t.Run("invalid json including null racksPerRow is rejected", func(t *testing.T) {
		withResourceHandler(t, &fakeResourceService{}, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "room", "/rooms/"+id.String(), `{"code":"ROOM-01","name":"机房","version":5,"racksPerRow":null}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_REQUEST")
		})
	})

	t.Run("zero version is surfaced as invalid resource", func(t *testing.T) {
		fake := &fakeResourceService{updateRoom: func(_ context.Context, _ service.Actor, _ uuid.UUID, version uint, _ service.RoomWriteInput) (model.Room, error) {
			if version != 0 {
				t.Fatalf("version = %d, want 0", version)
			}
			return model.Room{}, service.ErrInvalidResource
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "room", "/rooms/"+id.String(), `{"code":"ROOM-01","name":"机房","version":0}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE")
		})
	})

	t.Run("version conflict returns 409", func(t *testing.T) {
		fake := &fakeResourceService{updateRoom: func(context.Context, service.Actor, uuid.UUID, uint, service.RoomWriteInput) (model.Room, error) {
			return model.Room{}, service.ErrResourceVersion
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "room", "/rooms/"+id.String(), `{"code":"ROOM-01","name":"机房","version":5}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusConflict, "RESOURCE_VERSION_CONFLICT")
		})
	})
}

func TestUpdateRackHandler(t *testing.T) {
	id := uuid.New()
	register := func(router *gin.Engine, handler *ResourceHandler) {
		router.PUT("/racks/:id", handler.UpdateRack)
	}

	t.Run("success passes id version and payload", func(t *testing.T) {
		fake := &fakeResourceService{updateRack: func(_ context.Context, _ service.Actor, gotID uuid.UUID, version uint, item model.Rack) (model.Rack, error) {
			if gotID != id || version != 2 || item.Code != "rack-01" || item.UHeight != 42 {
				t.Fatalf("unexpected service arguments: id=%s version=%d item=%+v", gotID, version, item)
			}
			item.ID = gotID
			item.Version = version + 1
			return item, nil
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "rack", "/racks/"+id.String(), `{"code":"rack-01","name":"机柜","uHeight":42,"version":2}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusOK, "SUCCESS")
		})
	})

	t.Run("invalid id is rejected", func(t *testing.T) {
		withResourceHandler(t, &fakeResourceService{}, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "rack", "/racks/not-a-uuid", `{}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE_ID")
		})
	})

	t.Run("invalid json is rejected", func(t *testing.T) {
		withResourceHandler(t, &fakeResourceService{}, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "rack", "/racks/"+id.String(), `{`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_REQUEST")
		})
	})

	t.Run("zero version is surfaced as invalid resource", func(t *testing.T) {
		fake := &fakeResourceService{updateRack: func(_ context.Context, _ service.Actor, _ uuid.UUID, version uint, _ model.Rack) (model.Rack, error) {
			if version != 0 {
				t.Fatalf("version = %d, want 0", version)
			}
			return model.Rack{}, service.ErrInvalidResource
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "rack", "/racks/"+id.String(), `{"code":"RACK-01","name":"机柜","version":0}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusBadRequest, "INVALID_RESOURCE")
		})
	})

	t.Run("version conflict returns 409", func(t *testing.T) {
		fake := &fakeResourceService{updateRack: func(context.Context, service.Actor, uuid.UUID, uint, model.Rack) (model.Rack, error) {
			return model.Rack{}, service.ErrResourceVersion
		}}
		withResourceHandler(t, fake, func(handler *ResourceHandler) {
			recorder, envelope := performResourceUpdate(t, handler, "rack", "/racks/"+id.String(), `{"code":"RACK-01","name":"机柜","version":2}`, register)
			assertHandlerResponse(t, recorder, envelope, http.StatusConflict, "RESOURCE_VERSION_CONFLICT")
		})
	})
}

func TestCopyAndMoveResourceHandlers(t *testing.T) {
	sourceID, targetDCID, targetRoomID := uuid.New(), uuid.New(), uuid.New()
	fake := &fakeResourceService{
		copyRoom: func(_ context.Context, _ service.Actor, gotID uuid.UUID, input service.ResourceCopyInput) (service.ResourceCopyResult, error) {
			if gotID != sourceID || input.Version != 4 || input.TargetDataCenterID != targetDCID || !input.AutoGenerateCode || input.Name != "机房副本" {
				t.Fatalf("unexpected copy input: id=%s input=%+v", gotID, input)
			}
			return service.ResourceCopyResult{Resource: model.Room{BaseModel: model.BaseModel{ID: uuid.New()}, Name: input.Name}, CopiedRooms: 1, CopiedRacks: 2}, nil
		},
		moveRack: func(_ context.Context, _ service.Actor, gotID uuid.UUID, input service.ResourceMoveInput) (model.Rack, error) {
			if gotID != sourceID || input.Version != 6 || input.TargetRoomID != targetRoomID || input.Code != "RACK-NEW" {
				t.Fatalf("unexpected move input: id=%s input=%+v", gotID, input)
			}
			return model.Rack{BaseModel: model.BaseModel{ID: sourceID, Version: 7}, RoomID: targetRoomID, Code: input.Code}, nil
		},
	}
	withResourceHandler(t, fake, func(handler *ResourceHandler) {
		recorder, envelope := performResourceCreate(t, handler, "/rooms/"+sourceID.String()+"/copy", fmt.Sprintf(`{"version":4,"targetDataCenterId":"%s","name":"机房副本","autoGenerateCode":true}`, targetDCID), func(router *gin.Engine, handler *ResourceHandler) {
			router.POST("/rooms/:roomId/copy", handler.CopyRoom)
		})
		assertHandlerResponse(t, recorder, envelope, http.StatusCreated, "SUCCESS")
		recorder, envelope = performResourceCreate(t, handler, "/racks/"+sourceID.String()+"/move", fmt.Sprintf(`{"version":6,"targetRoomId":"%s","code":"RACK-NEW","autoGenerateCode":false}`, targetRoomID), func(router *gin.Engine, handler *ResourceHandler) {
			router.POST("/racks/:rackId/move", handler.MoveRack)
		})
		assertHandlerResponse(t, recorder, envelope, http.StatusOK, "SUCCESS")
	})
}
