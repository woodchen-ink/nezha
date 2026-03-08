package singleton

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nezhahq/nezha/model"
)

const autoCountryGroupNamePrefix = "[AUTO] Country: "

// AutoGroupAllServersByCountry 对所有已有 GeoIP 数据的服务器执行一次按国家自动分组。
// 用于配置变更时以及定时刷新场景。
func AutoGroupAllServersByCountry() {
	if !Conf.AutoGroupByCountry {
		return
	}

	serverList := ServerShared.GetList()
	var total, skippedNil, skippedEmpty, count int
	for _, server := range serverList {
		total++
		if server.GeoIP == nil {
			skippedNil++
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(server.GeoIP.CountryCode))
		if code == "" {
			skippedEmpty++
			continue
		}
		if err := AutoGroupServerByCountry(server, code); err != nil {
			log.Printf("NEZHA>> Auto group by country failed: %v, serverID: %d", err, server.ID)
		} else {
			count++
		}
	}

	log.Printf("NEZHA>> AutoGroupAll: total=%d processed=%d skippedNilGeoIP=%d skippedEmptyCode=%d",
		total, count, skippedNil, skippedEmpty)
}

func AutoGroupServerByCountry(server *model.Server, countryCode string) error {
	if server == nil || server.ID == 0 {
		return errors.New("invalid server")
	}

	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if code == "" {
		return nil
	}

	targetGroupName := fmt.Sprintf("%s%s", autoCountryGroupNamePrefix, code)
	return DB.Transaction(func(tx *gorm.DB) error {
		// 查找或创建目标国家分组
		var targetGroup model.ServerGroup
		err := tx.Where("name = ?", targetGroupName).First(&targetGroup).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("query group: %w", err)
			}
			targetGroup = model.ServerGroup{
				Common: model.Common{
					UserID: server.UserID,
				},
				Name: targetGroupName,
			}
			if err := tx.Create(&targetGroup).Error; err != nil {
				return fmt.Errorf("create group: %w", err)
			}
		}

		// 确保服务器属于目标国家分组
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ServerGroupServer{
			Common: model.Common{
				UserID: server.UserID,
			},
			ServerGroupId: targetGroup.ID,
			ServerId:      server.ID,
		}).Error; err != nil {
			return fmt.Errorf("link server to group: %w", err)
		}

		// 从其他自动国家分组中移除该服务器
		var otherAutoGroupIDs []uint64
		if err := tx.Model(&model.ServerGroup{}).
			Where("name LIKE ? AND id <> ?", autoCountryGroupNamePrefix+"%", targetGroup.ID).
			Pluck("id", &otherAutoGroupIDs).Error; err != nil {
			return fmt.Errorf("query other groups: %w", err)
		}
		if len(otherAutoGroupIDs) == 0 {
			return nil
		}

		if err := tx.Where("server_id = ? AND server_group_id IN (?)", server.ID, otherAutoGroupIDs).
			Delete(&model.ServerGroupServer{}).Error; err != nil {
			return fmt.Errorf("unlink from other groups: %w", err)
		}

		// 清理空的自动国家分组
		var activeGroupIDs []uint64
		if err := tx.Model(&model.ServerGroupServer{}).
			Distinct("server_group_id").
			Where("server_group_id IN (?)", otherAutoGroupIDs).
			Pluck("server_group_id", &activeGroupIDs).Error; err != nil {
			return fmt.Errorf("query active groups: %w", err)
		}

		active := make(map[uint64]struct{}, len(activeGroupIDs))
		for _, id := range activeGroupIDs {
			active[id] = struct{}{}
		}

		var staleGroupIDs []uint64
		for _, id := range otherAutoGroupIDs {
			if _, ok := active[id]; !ok {
				staleGroupIDs = append(staleGroupIDs, id)
			}
		}

		if len(staleGroupIDs) > 0 {
			if err := tx.Where("id IN (?)", staleGroupIDs).Delete(&model.ServerGroup{}).Error; err != nil {
				return fmt.Errorf("delete empty groups: %w", err)
			}
		}

		return nil
	})
}
