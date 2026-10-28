package service

import (
    "context"
    "errors"
    "fmt"
    "strings"
    "time"
    "omnirack-replica/backend/internal/model"
    "omnirack-replica/backend/internal/repository"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

var (
    ErrPDUHasChildren = errors.New("pdu has sockets or connections")
    ErrSocketConnected = errors.New("pdu socket is connected")
    ErrSocketUnavailable = errors.New("pdu socket is unavailable")
    ErrPDUDeviceRackMismatch = errors.New("device is not installed in this rack")
)

type PDUCreateInput struct { PDU model.PDU; AutoGenerateCode bool }
type PDUConnectionInput struct { Version uint `json:"version"`; DeviceID uuid.UUID `json:"deviceId"`; PowerW *float64 `json:"powerW"`; Circuit string `json:"circuit"`; RedundancyRole string `json:"redundancyRole"` }

type PDUService struct { store repository.PDUStore }
func NewPDUService(store repository.PDUStore) *PDUService { return &PDUService{store:store} }

func (s *PDUService) List(ctx context.Context, rackID uuid.UUID) ([]model.PDU,error) { if _,err:=s.store.GetRack(ctx,rackID);err!=nil{return nil,mapPDUStoreError(err)}; return s.store.ListPDUsByRack(ctx,rackID) }
func (s *PDUService) Get(ctx context.Context,id uuid.UUID)(model.PDU,error){x,e:=s.store.GetPDU(ctx,id);if e!=nil{return x,mapPDUStoreError(e)}; return x,nil}
func (s *PDUService) Create(ctx context.Context,actor Actor,rackID uuid.UUID,input PDUCreateInput)(model.PDU,error){
    _,e:=s.store.GetRack(ctx,rackID);if e!=nil{return model.PDU{},mapPDUStoreError(e)}
    x:=input.PDU;x.RackID=rackID
    if input.AutoGenerateCode || strings.TrimSpace(x.Code)=="" { x.Code=generateAutoCode("PDU") }
    normalizePDU(&x);if e=validatePDU(x);e!=nil{return model.PDU{},e}
    e=s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{if e:=st.CreatePDU(ctx,&x);e!=nil{return mapPDUStoreError(e)};return createAudit(ctx,st,actor,"CREATE","pdu",x.ID,nil,x)})
    return x,e
}
func (s *PDUService) Update(ctx context.Context,actor Actor,id uuid.UUID,expected uint,x model.PDU)(model.PDU,error){if expected==0{return model.PDU{},fmt.Errorf("%w: version 必须大于 0",ErrInvalidResource)};normalizePDU(&x);if e:=validatePDU(x);e!=nil{return model.PDU{},e};var out model.PDU;e:=s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{cur,e:=st.GetPDU(ctx,id);if e!=nil{return mapPDUStoreError(e)};if cur.Version!=expected{return ErrResourceVersion};x.ID,x.RackID,x.CreatedAt=cur.ID,cur.RackID,cur.CreatedAt;if e=st.UpdatePDU(ctx,&x,expected);e!=nil{return mapPDUStoreError(e)};out=x;return createAudit(ctx,st,actor,"UPDATE","pdu",id,cur,out)});return out,e}
func (s *PDUService) Delete(ctx context.Context,actor Actor,id uuid.UUID,expected uint)error{if expected==0{return fmt.Errorf("%w: version 必须大于 0",ErrInvalidResource)};return s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{cur,e:=st.GetPDU(ctx,id);if e!=nil{return mapPDUStoreError(e)};if cur.Version!=expected{return ErrResourceVersion};sockets,e:=st.ListSocketsByPDU(ctx,id);if e!=nil{return e};for _,socket:=range sockets{if _,e:=st.GetConnectionBySocket(ctx,socket.ID);e==nil{return ErrPDUHasChildren};if !errors.Is(e,gorm.ErrRecordNotFound){return e}};if len(sockets)>0{return ErrPDUHasChildren};if e=st.DeletePDU(ctx,id,expected);e!=nil{return mapPDUStoreError(e)};return createAudit(ctx,st,actor,"DELETE","pdu",id,cur,nil)})}

func (s *PDUService) ListSockets(ctx context.Context,pduID uuid.UUID)([]model.PDUSocket,error){if _,e:=s.store.GetPDU(ctx,pduID);e!=nil{return nil,mapPDUStoreError(e)};return s.store.ListSocketsByPDU(ctx,pduID)}
func (s *PDUService) CreateSocket(ctx context.Context,actor Actor,pduID uuid.UUID,x model.PDUSocket)(model.PDUSocket,error){if _,e:=s.store.GetPDU(ctx,pduID);e!=nil{return x,mapPDUStoreError(e)};x.PDUID=pduID;normalizeSocket(&x);if e:=validateSocket(x);e!=nil{return x,e};e:=s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{if e:=st.CreateSocket(ctx,&x);e!=nil{return mapPDUStoreError(e)};return createAudit(ctx,st,actor,"CREATE","pdu_socket",x.ID,nil,x)});return x,e}
func (s *PDUService) UpdateSocket(ctx context.Context,actor Actor,id uuid.UUID,expected uint,x model.PDUSocket)(model.PDUSocket,error){if expected==0{return x,fmt.Errorf("%w: version 必须大于 0",ErrInvalidResource)};normalizeSocket(&x);if e:=validateSocket(x);e!=nil{return x,e};var out model.PDUSocket;e:=s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{cur,e:=st.GetSocket(ctx,id);if e!=nil{return mapPDUStoreError(e)};if cur.Version!=expected{return ErrResourceVersion};if cur.PDUID!=x.PDUID{x.PDUID=cur.PDUID};if _,e=st.GetConnectionBySocket(ctx,id);e==nil && x.Status==model.PDUSocketStatusDisabled{return ErrSocketConnected};x.ID,x.PDUID,x.CreatedAt=cur.ID,cur.PDUID,cur.CreatedAt;if e=st.UpdateSocket(ctx,&x,expected);e!=nil{return mapPDUStoreError(e)};out=x;return createAudit(ctx,st,actor,"UPDATE","pdu_socket",id,cur,out)});return out,e}
func (s *PDUService) DeleteSocket(ctx context.Context,actor Actor,id uuid.UUID,expected uint)error{if expected==0{return fmt.Errorf("%w: version 必须大于 0",ErrInvalidResource)};return s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{cur,e:=st.GetSocket(ctx,id);if e!=nil{return mapPDUStoreError(e)};if cur.Version!=expected{return ErrResourceVersion};if _,e=st.GetConnectionBySocket(ctx,id);e==nil{return ErrSocketConnected};if !errors.Is(e,gorm.ErrRecordNotFound){return e};if e=st.DeleteSocket(ctx,id,expected);e!=nil{return mapPDUStoreError(e)};return createAudit(ctx,st,actor,"DELETE","pdu_socket",id,cur,nil)})}

func (s *PDUService) ListConnections(ctx context.Context,rackID uuid.UUID)([]model.PDUConnection,error){if _,e:=s.store.GetRack(ctx,rackID);e!=nil{return nil,mapPDUStoreError(e)};return s.store.ListConnectionsByRack(ctx,rackID)}
func (s *PDUService) Connect(ctx context.Context,actor Actor,socketID uuid.UUID,input PDUConnectionInput)(model.PDUConnection,error){
    socket,e:=s.store.GetSocket(ctx,socketID);if e!=nil{return model.PDUConnection{},mapPDUStoreError(e)}
    pdu,e:=s.store.GetPDU(ctx,socket.PDUID);if e!=nil{return model.PDUConnection{},mapPDUStoreError(e)}
    if socket.Status==model.PDUSocketStatusDisabled{return model.PDUConnection{},ErrSocketUnavailable}
    if _,e=s.store.GetConnectionBySocket(ctx,socketID);e==nil{return model.PDUConnection{},ErrSocketConnected}else if !errors.Is(e,gorm.ErrRecordNotFound){return model.PDUConnection{},e}
    device,e:=s.store.GetDevice(ctx,input.DeviceID);if e!=nil{return model.PDUConnection{},mapPDUStoreError(e)}
    pos,e:=s.store.GetPositionByDevice(ctx,device.ID);if e!=nil{return model.PDUConnection{},fmt.Errorf("%w: 设备必须先上架",ErrDeviceNotPositioned)}
    if pos.RackID!=pdu.RackID{return model.PDUConnection{},ErrPDUDeviceRackMismatch}
    role:=strings.ToUpper(strings.TrimSpace(input.RedundancyRole));if role==""{role=model.PDUConnectionPrimary};if role!=model.PDUConnectionPrimary&&role!=model.PDUConnectionRedundant{return model.PDUConnection{},fmt.Errorf("%w: 冗余角色必须为 PRIMARY 或 REDUNDANT",ErrInvalidResource)}
    x:=model.PDUConnection{SocketID:socketID,DeviceID:device.ID,PowerW:input.PowerW,Circuit:strings.TrimSpace(input.Circuit),RedundancyRole:role,ConnectedAt:time.Now().UTC(),ConnectedBy:actor.UserID}
    e=s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{if e:=st.CreateConnection(ctx,&x);e!=nil{return mapPDUStoreError(e)};if e:=st.SetSocketStatus(ctx,socketID,model.PDUSocketStatusConnected);e!=nil{return e};return createAudit(ctx,st,actor,"CREATE","pdu_connection",x.ID,nil,x)});return x,e
}
func (s *PDUService) Disconnect(ctx context.Context,actor Actor,id uuid.UUID,expected uint)error{if expected==0{return fmt.Errorf("%w: version 必须大于 0",ErrInvalidResource)};return s.store.WithinPDUTransaction(ctx,func(st repository.PDUStore)error{x,e:=st.GetConnection(ctx,id);if e!=nil{return mapPDUStoreError(e)};if x.Version!=expected{return ErrResourceVersion};if e=st.DeleteConnection(ctx,id,expected);e!=nil{return mapPDUStoreError(e)};if e=st.SetSocketStatus(ctx,x.SocketID,model.PDUSocketStatusAvailable);e!=nil{return e};return createAudit(ctx,st,actor,"DELETE","pdu_connection",id,x,nil)})}

func normalizePDU(x *model.PDU){x.Code=strings.ToUpper(strings.TrimSpace(x.Code));x.Name=strings.TrimSpace(x.Name);x.Manufacturer=strings.TrimSpace(x.Manufacturer);x.ModelNumber=strings.TrimSpace(x.ModelNumber);x.SerialNumber=strings.TrimSpace(x.SerialNumber);x.Status=strings.ToUpper(strings.TrimSpace(x.Status));if x.Status==""{x.Status=model.PDUStatusActive};x.Remarks=strings.TrimSpace(x.Remarks)}
func validatePDU(x model.PDU)error{if x.RackID==uuid.Nil{return fmt.Errorf("%w: 必须指定机柜",ErrInvalidResource)};if x.Code==""||len(x.Code)>80{return fmt.Errorf("%w: PDU 编码不能为空且不能超过 80 个字符",ErrInvalidResource)};if x.Name==""||len(x.Name)>150{return fmt.Errorf("%w: PDU 名称不能为空且不能超过 150 个字符",ErrInvalidResource)};if x.Status!=model.PDUStatusActive&&x.Status!=model.PDUStatusDisabled{return fmt.Errorf("%w: PDU 状态无效",ErrInvalidResource)};if x.InputVoltage!=nil&&*x.InputVoltage<=0||x.RatedPowerW!=nil&&*x.RatedPowerW<0||x.RatedCurrentA!=nil&&*x.RatedCurrentA<0{return fmt.Errorf("%w: PDU 电气参数无效",ErrInvalidResource)};return nil}
func normalizeSocket(x *model.PDUSocket){x.Standard=strings.ToUpper(strings.TrimSpace(x.Standard));x.Status=strings.ToUpper(strings.TrimSpace(x.Status));if x.Status==""{x.Status=model.PDUSocketStatusAvailable};x.Label=strings.TrimSpace(x.Label)}
func validateSocket(x model.PDUSocket)error{if x.PDUID==uuid.Nil{return fmt.Errorf("%w: 必须指定 PDU",ErrInvalidResource)};if x.SocketNo<1{return fmt.Errorf("%w: 插座编号必须大于 0",ErrInvalidResource)};if x.Standard!=model.PDUSocketStandardCN&&x.Standard!=model.PDUSocketStandardEU{return fmt.Errorf("%w: 插座制式必须为 CN 或 EU",ErrInvalidResource)};if x.AmperageA!=10&&x.AmperageA!=16{return fmt.Errorf("%w: 插座电流必须为 10A 或 16A",ErrInvalidResource)};if x.Status!=model.PDUSocketStatusAvailable&&x.Status!=model.PDUSocketStatusConnected&&x.Status!=model.PDUSocketStatusDisabled{return fmt.Errorf("%w: 插座状态无效",ErrInvalidResource)};return nil}
func mapPDUStoreError(e error)error{if errors.Is(e,gorm.ErrRecordNotFound){return ErrResourceNotFound};if errors.Is(e,repository.ErrVersionConflict){return ErrResourceVersion};return e}

