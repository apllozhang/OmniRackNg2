package handler

import (
    "errors"
    "net/http"
    "omnirack-replica/backend/internal/model"
    "omnirack-replica/backend/internal/response"
    "omnirack-replica/backend/internal/service"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type PDUHandler struct{ service *service.PDUService }
func NewPDUHandler(svc *service.PDUService)*PDUHandler{return &PDUHandler{service:svc}}

type pduCreateRequest struct{ model.PDU; AutoGenerateCode bool `json:"autoGenerateCode"` }
func (h *PDUHandler) List(c *gin.Context){id,ok:=parsePDUID(c,"rackId");if !ok{return};xs,e:=h.service.List(c.Request.Context(),id);if e!=nil{writePDUError(c,e);return};response.Success(c,gin.H{"items":xs})}
func (h *PDUHandler) Create(c *gin.Context){rackID,ok:=parsePDUID(c,"rackId");if !ok{return};var req pduCreateRequest;if c.ShouldBindJSON(&req)!=nil{response.Error(c,400,"INVALID_REQUEST","PDU 参数格式无效",nil);return};x,e:=h.service.Create(c.Request.Context(),actorFromContext(c),rackID,service.PDUCreateInput{PDU:req.PDU,AutoGenerateCode:req.AutoGenerateCode});if e!=nil{writePDUError(c,e);return};response.Created(c,x)}
func (h *PDUHandler) Get(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};x,e:=h.service.Get(c.Request.Context(),id);if e!=nil{writePDUError(c,e);return};response.Success(c,x)}
func (h *PDUHandler) Update(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};var x model.PDU;if c.ShouldBindJSON(&x)!=nil{response.Error(c,400,"INVALID_REQUEST","PDU 参数格式无效",nil);return};x.ID=id;y,e:=h.service.Update(c.Request.Context(),actorFromContext(c),id,x.Version,x);if e!=nil{writePDUError(c,e);return};response.Success(c,y)}
func (h *PDUHandler) Delete(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};v,ok:=parseVersion(c);if !ok{return};if e:=h.service.Delete(c.Request.Context(),actorFromContext(c),id,v);e!=nil{writePDUError(c,e);return};response.Success(c,gin.H{"deleted":true,"id":id})}
func (h *PDUHandler) ListSockets(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};xs,e:=h.service.ListSockets(c.Request.Context(),id);if e!=nil{writePDUError(c,e);return};response.Success(c,gin.H{"items":xs})}
func (h *PDUHandler) CreateSocket(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};var x model.PDUSocket;if c.ShouldBindJSON(&x)!=nil{response.Error(c,400,"INVALID_REQUEST","插座参数格式无效",nil);return};x.PDUID=id;y,e:=h.service.CreateSocket(c.Request.Context(),actorFromContext(c),id,x);if e!=nil{writePDUError(c,e);return};response.Created(c,y)}
func (h *PDUHandler) UpdateSocket(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};var x model.PDUSocket;if c.ShouldBindJSON(&x)!=nil{response.Error(c,400,"INVALID_REQUEST","插座参数格式无效",nil);return};x.ID=id;y,e:=h.service.UpdateSocket(c.Request.Context(),actorFromContext(c),id,x.Version,x);if e!=nil{writePDUError(c,e);return};response.Success(c,y)}
func (h *PDUHandler) DeleteSocket(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};v,ok:=parseVersion(c);if !ok{return};if e:=h.service.DeleteSocket(c.Request.Context(),actorFromContext(c),id,v);e!=nil{writePDUError(c,e);return};response.Success(c,gin.H{"deleted":true,"id":id})}
func (h *PDUHandler) ListConnections(c *gin.Context){id,ok:=parsePDUID(c,"rackId");if !ok{return};xs,e:=h.service.ListConnections(c.Request.Context(),id);if e!=nil{writePDUError(c,e);return};response.Success(c,gin.H{"items":xs})}
func (h *PDUHandler) Connect(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};var x service.PDUConnectionInput;if c.ShouldBindJSON(&x)!=nil{response.Error(c,400,"INVALID_REQUEST","供电连接参数格式无效",nil);return};y,e:=h.service.Connect(c.Request.Context(),actorFromContext(c),id,x);if e!=nil{writePDUError(c,e);return};response.Created(c,y)}
func (h *PDUHandler) Disconnect(c *gin.Context){id,ok:=parsePDUID(c,"id");if !ok{return};var x struct{Version uint `json:"version"`};if c.ShouldBindJSON(&x)!=nil{response.Error(c,400,"INVALID_REQUEST","连接版本不能为空",nil);return};if e:=h.service.Disconnect(c.Request.Context(),actorFromContext(c),id,x.Version);e!=nil{writePDUError(c,e);return};response.Success(c,gin.H{"deleted":true,"id":id})}
func parsePDUID(c *gin.Context,name string)(uuid.UUID,bool){id,e:=uuid.Parse(c.Param(name));if e!=nil{response.Error(c,http.StatusBadRequest,"INVALID_RESOURCE_ID","资源 ID 格式无效",nil);return uuid.Nil,false};return id,true}
func writePDUError(c *gin.Context,e error){switch{case errors.Is(e,service.ErrResourceNotFound):response.Error(c,404,"RESOURCE_NOT_FOUND","资源不存在或已删除",nil);case errors.Is(e,service.ErrDuplicateCode):response.Error(c,409,"RESOURCE_CODE_DUPLICATE","PDU 编码已存在",nil);case errors.Is(e,service.ErrResourceVersion):response.Error(c,409,"RESOURCE_VERSION_CONFLICT","数据已被其他用户修改，请刷新后重试",nil);case errors.Is(e,service.ErrInvalidResource):response.Error(c,400,"INVALID_RESOURCE",e.Error(),nil);case errors.Is(e,service.ErrPDUHasChildren):response.Error(c,409,"PDU_HAS_CHILDREN","请先删除插座和供电连接后再删除 PDU",nil);case errors.Is(e,service.ErrSocketConnected):response.Error(c,409,"PDU_SOCKET_CONNECTED","插座已有供电连接",nil);case errors.Is(e,service.ErrSocketUnavailable):response.Error(c,409,"PDU_SOCKET_UNAVAILABLE","插座不可用",nil);case errors.Is(e,service.ErrPDUDeviceRackMismatch):response.Error(c,409,"PDU_DEVICE_RACK_MISMATCH","设备必须已上架在当前 PDU 所属机柜",nil);case errors.Is(e,service.ErrDeviceNotPositioned):response.Error(c,409,"DEVICE_NOT_POSITIONED","设备必须先上架后才能建立供电连接",nil);default:_=c.Error(e);response.Error(c,500,"INTERNAL_ERROR","系统内部错误",nil)}}
