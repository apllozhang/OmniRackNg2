package service

import (
	"fmt"
	"sort"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Merge ranges rather than count devices; a 4U device occupies four physical units.
func usedRackUnits(capacity int, positions []model.RackUOccupancy) int {
	if capacity <= 0 {
		return 0
	}
	ranges := [][2]int{}
	for _, p := range positions {
		start, end := max(1, p.StartU), min(capacity, p.EndU)
		if start <= end {
			ranges = append(ranges, [2]int{start, end})
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })
	total, end := 0, 0
	for _, p := range ranges {
		start := max(end+1, p[0])
		if start <= p[1] {
			total += p[1] - start + 1
			end = p[1]
		}
	}
	return total
}

func rackUSeverity(used, capacity int, settings model.NotificationSettings) string {
	if capacity <= 0 {
		return ""
	}
	if used*100 >= capacity*settings.RackCriticalPercent {
		return "DANGER"
	}
	if used*100 >= capacity*settings.RackWarningPercent {
		return "WARNING"
	}
	return ""
}

func refreshRackUReminders(tx *gorm.DB, put func(model.AssetReminder) error) error {
	settings, err := ReadNotificationSettings(tx)
	if err != nil {
		return err
	}
	racks := []model.Rack{}
	if err := tx.Where("status IN ?", []string{model.RackStatusAvailable, model.RackStatusPartial, model.RackStatusFull}).Find(&racks).Error; err != nil {
		return err
	}
	if len(racks) == 0 {
		return nil
	}
	positions := []model.RackUOccupancy{}
	if err := tx.Find(&positions).Error; err != nil {
		return err
	}
	byRack := map[uuid.UUID][]model.RackUOccupancy{}
	for _, p := range positions {
		byRack[p.RackID] = append(byRack[p.RackID], p)
	}
	rooms := []model.Room{}
	centers := []model.DataCenter{}
	if err := tx.Where("status = ?", model.ResourceStatusOperating).Find(&rooms).Error; err != nil {
		return err
	}
	if err := tx.Where("status = ?", model.ResourceStatusOperating).Find(&centers).Error; err != nil {
		return err
	}
	roomByID := map[uuid.UUID]model.Room{}
	centerByID := map[uuid.UUID]model.DataCenter{}
	for _, r := range rooms {
		roomByID[r.ID] = r
	}
	for _, c := range centers {
		centerByID[c.ID] = c
	}
	for _, rack := range racks {
		room, roomOK := roomByID[rack.RoomID]
		center, centerOK := centerByID[rack.DataCenterID]
		if !roomOK || !centerOK || rack.UHeight <= 0 {
			continue
		}
		used := usedRackUnits(rack.UHeight, byRack[rack.ID])
		severity := rackUSeverity(used, rack.UHeight, settings)
		if severity == "" {
			continue
		}
		title := "U 位占用率预警"
		if severity == "DANGER" {
			title = "U 位容量紧张"
		}
		if err := put(model.AssetReminder{Key: "racku:" + rack.ID.String(), Kind: "RACK_U", Title: rack.Name + " · " + title, Detail: fmt.Sprintf("%s / %s / %s：已用 %d/%dU，剩余 %dU，占用 %.1f%%", center.Name, room.Name, rack.Code, used, rack.UHeight, rack.UHeight-used, float64(used)*100/float64(rack.UHeight)), Severity: severity}); err != nil {
			return err
		}
	}
	return nil
}
