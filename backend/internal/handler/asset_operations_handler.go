package handler

import (
	"encoding/json"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strings"
	"time"
)

type AssetOperationsHandler struct{ ops *service.AssetOperations }

func NewAssetOperationsHandler(ops *service.AssetOperations) *AssetOperationsHandler {
	return &AssetOperationsHandler{ops: ops}
}

func (h *AssetOperationsHandler) currentRoles(c *gin.Context) ([]string, bool) {
	id, e := uuid.Parse(c.GetString(middleware.UserIDKey))
	if e != nil {
		response.Error(c, 401, "UNAUTHORIZED", "登录身份无效", nil)
		c.Abort()
		return nil, false
	}
	var user model.User
	if e := h.ops.DB.WithContext(c.Request.Context()).Preload("Roles").First(&user, "id = ? AND enabled = true", id).Error; e != nil {
		response.Error(c, 401, "UNAUTHORIZED", "账号已停用或身份无法验证", nil)
		c.Abort()
		return nil, false
	}
	roles := []string{}
	for _, role := range user.Roles {
		roles = append(roles, role.Code)
	}
	c.Set(middleware.RoleKey, roles)
	return roles, true
}

// LegacyAccess preserves established user capabilities while ensuring a newly added
// audit-only role cannot gain write access through old resource endpoints.
func (h *AssetOperationsHandler) LegacyAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, ok := h.currentRoles(c)
		if !ok {
			return
		}
		auditOnly := middleware.HasRole(roles, "asset_auditor") && !middleware.HasRole(roles, "asset_manager") && !middleware.HasRole(roles, "system_admin")
		if auditOnly && c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.FullPath() != "/api/v1/auth/logout" {
			response.Error(c, 403, "FORBIDDEN", "审计角色不能修改资源", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentAccess revalidates role/disabled status in the database, rather than trusting old JWT roles.
func (h *AssetOperationsHandler) CurrentAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, ok := h.currentRoles(c)
		if !ok {
			return
		}
		canRead := middleware.HasRole(roles, "system_admin") || middleware.HasRole(roles, "asset_manager") || middleware.HasRole(roles, "asset_auditor")
		canWrite := middleware.HasRole(roles, "system_admin") || middleware.HasRole(roles, "asset_manager")
		readOnlyExport := c.Request.Method == "POST" && c.FullPath() == "/api/v1/asset-ops/export"
		if !canRead || (c.Request.Method != "GET" && c.Request.Method != "HEAD" && !readOnlyExport && !canWrite) {
			response.Error(c, 403, "FORBIDDEN", "需要资产管理或审计权限", nil)
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
		c.Next()
	}
}

func (h *AssetOperationsHandler) Sites(c *gin.Context) {
	items := []model.AssetSite{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Order("region,name").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) CreateSite(c *gin.Context) {
	var in model.AssetSite
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.CreateSite(c.Request.Context(), actorFromContext(c), in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Created(c, x)
}
func (h *AssetOperationsHandler) Devices(c *gin.Context) {
	items, e := h.ops.Overview(c.Request.Context())
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) Profile(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in model.AssetProfile
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.SaveProfile(c.Request.Context(), actorFromContext(c), id, in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) Coverage(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in model.AssetCoverage
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	in.DeviceID = id
	x, e := h.ops.AddCoverage(c.Request.Context(), actorFromContext(c), in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Created(c, x)
}
func (h *AssetOperationsHandler) Specs(c *gin.Context) {
	items := []model.SpareSpec{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Order("category,name").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) CreateSpec(c *gin.Context) {
	var in model.SpareSpec
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.CreateSpec(c.Request.Context(), actorFromContext(c), in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Created(c, x)
}
func (h *AssetOperationsHandler) Stocks(c *gin.Context) {
	items := []model.SpareStock{}
	q := h.ops.DB.WithContext(c.Request.Context()).Preload("Spec").Preload("Site").Preload("InstalledDevice")
	if raw := c.Query("siteId"); raw != "" {
		id, e := uuid.Parse(raw)
		if e != nil {
			invalidJSON(c)
			return
		}
		q = q.Where("site_id = ?", id)
	}
	if e := q.Order("created_at DESC").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) CreateStock(c *gin.Context) {
	var in model.SpareStock
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.CreateStock(c.Request.Context(), actorFromContext(c), in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Created(c, x)
}

func (h *AssetOperationsHandler) ReceiveSpare(c *gin.Context) {
	var in service.SpareReceiptInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.ReceiveSpare(c.Request.Context(), actorFromContext(c), in)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) MoveStock(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in service.SpareMoveInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.MoveStock(c.Request.Context(), actorFromContext(c), id, in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) Movements(c *gin.Context) {
	items := []model.SpareMovement{}
	q := h.ops.DB.WithContext(c.Request.Context()).Preload("Stock.Spec").Preload("Stock.Site")
	if raw := c.Query("stockId"); raw != "" {
		id, e := uuid.Parse(raw)
		if e != nil {
			invalidJSON(c)
			return
		}
		q = q.Where("stock_id = ?", id)
	}
	if e := q.Order("created_at DESC").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) Minimums(c *gin.Context) {
	items := []model.SpareMinimum{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Preload("Spec").Preload("Site").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) SaveMinimum(c *gin.Context) {
	var in model.SpareMinimum
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	if in.Minimum < 0 || in.SpecID == uuid.Nil || in.SiteID == uuid.Nil {
		writeResourceError(c, service.ErrInvalidResource)
		return
	}
	var out model.SpareMinimum
	e := h.ops.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var site model.AssetSite
		var spec model.SpareSpec
		if e := tx.First(&site, "id = ?", in.SiteID).Error; e != nil {
			return e
		}
		if e := tx.First(&spec, "id = ?", in.SpecID).Error; e != nil {
			return e
		}
		in.BaseModel = model.BaseModel{}
		in.Spec = model.SpareSpec{}
		in.Site = model.AssetSite{}
		if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "spec_id"}, {Name: "site_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"minimum": in.Minimum, "version": gorm.Expr("spare_minimums.version + 1"), "updated_at": time.Now().UTC()})}).Omit("Spec", "Site").Create(&in).Error; e != nil {
			return e
		}
		if e := tx.Where("spec_id = ? AND site_id = ?", in.SpecID, in.SiteID).First(&out).Error; e != nil {
			return e
		}
		return h.ops.Audit(tx, actorFromContext(c), "SET_MINIMUM", "spare_minimum", out.ID, nil, out)
	})
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, out)
}
func (h *AssetOperationsHandler) Checks(c *gin.Context) {
	items := []model.AssetCheckTask{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Preload("Site").Order("created_at DESC").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) CreateCheck(c *gin.Context) {
	var in service.CheckCreateInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.CreateCheck(c.Request.Context(), actorFromContext(c), in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Created(c, x)
}
func (h *AssetOperationsHandler) Lines(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items := []model.AssetCheckLine{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Where("task_id = ?", id).Order("kind,name").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) Scan(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in service.CheckScanInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, duplicate, e := h.ops.ScanCheck(c.Request.Context(), actorFromContext(c), id, in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"line": x, "duplicate": duplicate})
}
func (h *AssetOperationsHandler) Resolve(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	lineID, ok := parseID(c, "lineId")
	if !ok {
		return
	}
	var in service.CheckResolveInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, e := h.ops.ResolveCheck(c.Request.Context(), actorFromContext(c), id, lineID, in)
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) Close(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if e := h.ops.CloseCheck(c.Request.Context(), actorFromContext(c), id); e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"closed": true})
}
func (h *AssetOperationsHandler) Discoveries(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items := []model.AssetCheckDiscovery{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Where("task_id = ?", id).Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) Discover(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
		Note string `json:"note"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Code) == "" || len(in.Code) > 500 || strings.TrimSpace(in.Note) == "" {
		invalidJSON(c)
		return
	}
	actor := actorFromContext(c)
	x := model.AssetCheckDiscovery{TaskID: id, Code: strings.TrimSpace(in.Code), Note: strings.TrimSpace(in.Note), ActorID: *actor.UserID}
	e := h.ops.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var task model.AssetCheckTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ? AND state = 'ACTIVE' AND assignee_id = ?", id, actor.UserID).Error; e != nil {
			return service.ErrInvalidResource
		}
		if e := tx.Create(&x).Error; e != nil {
			return e
		}
		return h.ops.Audit(tx, actor, "DISCOVER", "asset_check", id, nil, x)
	})
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Created(c, x)
}
func (h *AssetOperationsHandler) ResolveDiscovery(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	discoveryID, ok := parseID(c, "discoveryId")
	if !ok {
		return
	}
	var in service.CheckResolveInput
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Resolution) == "" {
		invalidJSON(c)
		return
	}
	actor := actorFromContext(c)
	e := h.ops.DB.Transaction(func(tx *gorm.DB) error {
		var task model.AssetCheckTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ? AND state = 'ACTIVE'", id).Error; e != nil {
			return e
		}
		var x model.AssetCheckDiscovery
		if e := tx.First(&x, "id = ? AND task_id = ?", discoveryID, id).Error; e != nil {
			return e
		}
		if x.ActorID == *actor.UserID || x.ResolvedBy != nil {
			return service.ErrInvalidResource
		}
		x.ResolvedBy = actor.UserID
		x.Resolution = in.Resolution
		x.Version++
		if e := tx.Save(&x).Error; e != nil {
			return e
		}
		return h.ops.Audit(tx, actor, "RESOLVE_DISCOVERY", "asset_check", id, nil, x)
	})
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"resolved": true})
}
func (h *AssetOperationsHandler) Reminders(c *gin.Context) {
	items := []model.AssetReminder{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Where("active = true").Order("severity,created_at DESC").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AssetOperationsHandler) Refresh(c *gin.Context) {
	if e := h.ops.RefreshReminders(c.Request.Context()); e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"refreshed": true})
}
func (h *AssetOperationsHandler) Acknowledge(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Note string `json:"note"`
	}
	if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.Note) == "" {
		invalidJSON(c)
		return
	}
	actor := actorFromContext(c)
	e := h.ops.DB.Transaction(func(tx *gorm.DB) error {
		var x model.AssetReminder
		if e := tx.First(&x, "id = ?", id).Error; e != nil {
			return e
		}
		before := x
		x.AcknowledgedBy = actor.UserID
		x.Acknowledgement = in.Note
		x.Version++
		if e := tx.Save(&x).Error; e != nil {
			return e
		}
		return h.ops.Audit(tx, actor, "ACKNOWLEDGE", "asset_reminder", id, before, x)
	})
	if e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"acknowledged": true})
}
func (h *AssetOperationsHandler) Operators(c *gin.Context) {
	items := []model.User{}
	if e := h.ops.DB.WithContext(c.Request.Context()).Preload("Roles").Where("enabled = true").Order("username").Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	type row struct {
		ID       uuid.UUID `json:"id"`
		Name     string    `json:"name"`
		CanCheck bool      `json:"canCheck"`
	}
	out := []row{}
	for _, u := range items {
		r := row{ID: u.ID, Name: u.DisplayName}
		for _, role := range u.Roles {
			if role.Code == "system_admin" || role.Code == "asset_manager" {
				r.CanCheck = true
			}
		}
		out = append(out, r)
	}
	response.Success(c, gin.H{"items": out})
}
func (h *AssetOperationsHandler) Export(c *gin.Context) {
	devices, e := h.ops.Overview(c.Request.Context())
	if e != nil {
		writeResourceError(c, e)
		return
	}
	bundle := map[string]interface{}{"format": "cabinet-asset-export-v1", "generatedAt": time.Now().UTC(), "devices": devices}
	lists := map[string]interface{}{"pcHandovers": &[]model.PCHandover{}, "sites": &[]model.AssetSite{}, "specs": &[]model.SpareSpec{}, "stocks": &[]model.SpareStock{}, "movements": &[]model.SpareMovement{}, "minimums": &[]model.SpareMinimum{}, "checks": &[]model.AssetCheckTask{}, "checkLines": &[]model.AssetCheckLine{}, "discoveries": &[]model.AssetCheckDiscovery{}, "reminders": &[]model.AssetReminder{}}
	for name, dst := range lists {
		if e := h.ops.DB.WithContext(c.Request.Context()).Find(dst).Error; e != nil {
			writeResourceError(c, e)
			return
		}
		bundle[name] = dst
	}
	content, e := json.MarshalIndent(bundle, "", "  ")
	if e != nil {
		writeResourceError(c, e)
		return
	}
	if e := h.ops.Audit(h.ops.DB, actorFromContext(c), "EXPORT", "asset_operations", uuid.Nil, nil, gin.H{"devices": len(devices)}); e != nil {
		writeResourceError(c, e)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=asset-operations-"+time.Now().Format("20060102")+".json")
	c.Data(200, "application/json; charset=utf-8", content)
}

func (h *AssetOperationsHandler) Audit(c *gin.Context) {
	page := queryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	query := h.ops.DB.WithContext(c.Request.Context()).Model(&model.AuditLog{}).Where("resource_type IN ?", []string{
		"asset_site", "device", "spare_spec", "spare_stock", "spare_minimum", "asset_check", "asset_check_line", "asset_reminder", "asset_operations",
	})
	var total int64
	if e := query.Count(&total).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	items := []model.AuditLog{}
	if e := query.Order("created_at DESC").Limit(50).Offset((page - 1) * 50).Find(&items).Error; e != nil {
		writeResourceError(c, e)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "pageSize": 50})
}
