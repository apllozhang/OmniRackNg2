package integration

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"omnirack-replica/backend/internal/service"
)

func TestDeviceTableGlobalSortingScopeFiltersAndEmptyValues(t *testing.T) {
	db := isolatedPostgres(t)
	types := []model.DeviceType{{Code: "TEST_A", Name: "A-server", Category: "SERVER"}, {Code: "TEST_B", Name: "B-server", Category: "SERVER"}, {Code: "TEST_PC", Name: "C-terminal", Category: "PC"}}
	for i := range types {
		if err := db.Create(&types[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	devices := []model.Device{}
	ids := []string{}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 61; i >= 1; i-- {
		d := model.Device{Code: fmt.Sprintf("DEV-%03d", i), Name: fmt.Sprintf("DEV-%03d", i), TypeID: types[i%2].ID, HeightU: i%10 + 1, LifecycleStatus: "WAITING_RACK"}
		if i%2 == 0 {
			d.LifecycleStatus = "MAINTENANCE"
		}
		if i == 1 {
			d.AssetNumber = "NONEMPTY"
			d.ManagementIP = "10.0.0.1"
		}
		d.ID = uuid.New()
		d.CreatedAt = at
		if err := db.Create(&d).Error; err != nil {
			t.Fatal(err)
		}
		devices = append(devices, d)
		ids = append(ids, d.ID.String())
	}
	pc := model.Device{Code: "DEV-PC", Name: "DEV-PC", TypeID: types[2].ID, HeightU: 1}
	if err := db.Create(&pc).Error; err != nil {
		t.Fatal(err)
	}
	dc := model.DataCenter{Code: "QUERY-DC", Name: "Query"}
	if err := db.Create(&dc).Error; err != nil {
		t.Fatal(err)
	}
	room := model.Room{DataCenterID: dc.ID, Code: "QUERY-ROOM", Name: "Query"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatal(err)
	}
	racks := []model.Rack{{DataCenterID: dc.ID, RoomID: room.ID, Code: "A", Name: "A"}, {DataCenterID: dc.ID, RoomID: room.ID, Code: "B", Name: "B"}}
	for i := range racks {
		if err := db.Create(&racks[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		rack := racks[0]
		start := i*10 + 1
		if i == 3 {
			rack = racks[1]
			start = 1
		}
		p := model.RackDevicePosition{DeviceID: devices[i].ID, RackID: rack.ID, RoomID: room.ID, DataCenterID: dc.ID, StartU: start, HeightU: devices[i].HeightU, EndU: start + devices[i].HeightU - 1, InstalledAt: at}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.GET("/devices", handler.NewDeviceHandler(service.NewDeviceService(repository.NewDeviceStore(db))).ListDevices)
	get := func(query string, status int) service.DeviceListResult {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/devices?"+query, nil))
		if w.Code != status {
			t.Fatalf("%s: got%d expected%d %s", query, w.Code, status, w.Body.String())
		}
		var envelope struct {
			Data service.DeviceListResult `json:"data"`
		}
		if status == 200 {
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
		}
		return envelope.Data
	}
	for _, direction := range []string{"asc", "desc"} {
		seen := map[string]bool{}
		for page := 1; page <= 4; page++ {
			result := get(fmt.Sprintf("scope=ROOM&search=DEV-&sortBy=name&sortOrder=%s&pageSize=20&page=%d", direction, page), 200)
			if result.Total != 61 {
				t.Fatalf("room scope total%d", result.Total)
			}
			for j, d := range result.Items {
				index := (page-1)*20 + j + 1
				if direction == "desc" {
					index = 62 - index
				}
				if d.Name != fmt.Sprintf("DEV-%03d", index) {
					t.Fatalf("global order %s", d.Name)
				}
				if seen[d.ID.String()] {
					t.Fatal("duplicate page row")
				}
				seen[d.ID.String()] = true
			}
		}
	}
	sort.Strings(ids)
	ties := get("scope=ROOM&sortBy=createdAt&sortOrder=desc&pageSize=100", 200)
	for i, d := range ties.Items {
		if d.ID.String() != ids[i] {
			t.Fatal("unstable createdAt tie")
		}
	}
	for field := range repository.DeviceSortExpressions() {
		for _, direction := range []string{"asc", "desc"} {
			result := get("scope=ROOM&pageSize=100&sortBy="+field+"&sortOrder="+direction, 200)
			if result.Total != 61 || len(result.Items) != 61 {
				t.Fatal("sort changed membership")
			}
		}
	}
	for _, field := range []string{"assetNumber", "managementIp"} {
		for _, direction := range []string{"asc", "desc"} {
			rows := get("scope=ROOM&sortBy="+field+"&sortOrder="+direction, 200).Items
			if rows[0].Name != "DEV-001" {
				t.Fatal("empty strings were not last")
			}
		}
	}
	ascending := get("scope=ROOM&sortBy=location&sortOrder=asc", 200).Items
	descending := get("scope=ROOM&sortBy=location&sortOrder=desc", 200).Items
	if ascending[0].ID != devices[0].ID || descending[0].ID != devices[3].ID || ascending[4].CurrentPosition != nil || descending[4].CurrentPosition != nil {
		t.Fatal("location ordering or null position placement")
	}
	filtered := get("scope=ROOM&typeId="+types[0].ID.String()+"&lifecycleStatus=MAINTENANCE&sortBy=name&sortOrder=asc&pageSize=100", 200)
	if filtered.Total != 30 {
		t.Fatalf("combined type/status count%d", filtered.Total)
	}
	if get("scope=PC&search=DEV-&sortBy=name&sortOrder=asc", 200).Total != 1 {
		t.Fatal("PC scope leak")
	}
	get("sortBy="+url.QueryEscape("name;DROP TABLE users")+"&sortOrder=asc", 400)
	get("sortBy=name&sortOrder="+url.QueryEscape("asc;SELECT 1"), 400)
	get("sortBy=name", 400)
	get("sortOrder=asc", 400)
	if get("scope=ROOM&search=missing", 200).Total != 0 {
		t.Fatal("empty search")
	}
}
