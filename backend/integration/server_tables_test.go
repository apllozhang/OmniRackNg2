package integration

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/service"
)

// Executes real HTTP handlers and PostgreSQL queries in the owned random schema.
// Authentication/role gates are covered separately by the live HTTP runner.
func TestServerTablesSortBeforePagingAndFilterWholeCollection(t *testing.T) {
	db := isolatedPostgres(t)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	auditIDs := []string{}
	for i := 61; i >= 1; i-- {
		a := model.AuditLog{Action: "CREATE", ResourceType: "device", RequestID: fmt.Sprintf("AUDIT_%%_%03d", i), BeforeJSON: "{}", AfterJSON: "{}", Result: "SUCCESS"}
		a.ID = uuid.New()
		a.CreatedAt = at
		auditIDs = append(auditIDs, a.ID.String())
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
		n := model.NotificationDelivery{Channel: "EMAIL", Status: "SUCCESS", Summary: fmt.Sprintf("MSG_%%_%03d", i), Recipients: []string{"fixture@example.invalid"}}
		n.ID = uuid.New()
		n.CreatedAt = at
		if err := db.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
	}
	decoy := model.NotificationDelivery{Channel: "EMAIL", Status: "FAILED", Summary: "MSG_XY_001"}
	if err := db.Create(&decoy).Error; err != nil {
		t.Fatal(err)
	}
	ah := handler.NewAssetOperationsHandler(service.NewAssetOperations(db))
	nh := handler.NewNotificationHandler(service.NewNotificationService(db, "disposable-fixture"))
	router := gin.New()
	router.GET("/audit", ah.Audit)
	router.GET("/deliveries", nh.Deliveries)
	get := func(path string, status int) map[string]interface{} {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status {
			t.Fatalf("%s: got%d want%d: %s", path, w.Code, status, w.Body.String())
		}
		var envelope map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if status != 200 {
			return nil
		}
		return envelope["data"].(map[string]interface{})
	}
	for _, spec := range []struct{ path, search, key, field, prefix string }{
		{"/audit", "AUDIT_%_", "requestId", "requestId", "AUDIT_%_"},
		{"/deliveries", "MSG_%_", "summary", "summary", "MSG_%_"},
	} {
		seen := map[string]bool{}
		for page := 1; page <= 4; page++ {
			path := fmt.Sprintf("%s?page=%d&pageSize=20&sortBy=%s&sortOrder=asc&search=%s", spec.path, page, spec.key, url.QueryEscape(spec.search))
			data := get(path, 200)
			if data["total"].(float64) != 61 {
				t.Fatalf("filtered total: %v", data["total"])
			}
			items := data["items"].([]interface{})
			expected := 20
			if page == 4 {
				expected = 1
			}
			if len(items) != expected {
				t.Fatalf("page%d rows%d", page, len(items))
			}
			for j, raw := range items {
				row := raw.(map[string]interface{})
				want := fmt.Sprintf("%s%03d", spec.prefix, (page-1)*20+j+1)
				if row[spec.field] != want {
					t.Fatalf("global order: got%v want%s", row[spec.field], want)
				}
				id := row["id"].(string)
				if seen[id] {
					t.Fatal("duplicate across pages")
				}
				seen[id] = true
			}
		}
		descending := get(spec.path+"?pageSize=20&sortBy="+spec.key+"&sortOrder=desc&search="+url.QueryEscape(spec.search), 200)
		if descending["items"].([]interface{})[0].(map[string]interface{})[spec.field] != spec.prefix+"061" {
			t.Fatal("descending order not global")
		}
		if get(spec.path+"?search=nonexistent", 200)["total"].(float64) != 0 {
			t.Fatal("empty filter")
		}
		for _, query := range []string{"page=0", "page=10001", "pageSize=0", "pageSize=101", "sortBy=id%3BDROP+TABLE+users&sortOrder=asc", "sortBy=createdAt&sortOrder=asc%3BSELECT+1", "sortOrder=asc", "sortBy=createdAt", "search=" + strings.Repeat("x", 201)} {
			get(spec.path+"?"+query, 400)
		}
	}
	sort.Strings(auditIDs)
	// All timestamps are equal: ID tie-breaker must preserve order across pages.
	for page := 1; page <= 4; page++ {
		data := get(fmt.Sprintf("/audit?page=%d&pageSize=20&sortBy=createdAt&sortOrder=desc", page), 200)
		for j, raw := range data["items"].([]interface{}) {
			if raw.(map[string]interface{})["id"] != auditIDs[(page-1)*20+j] {
				t.Fatal("unstable timestamp tie order")
			}
		}
	}
}
