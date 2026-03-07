package singleton

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/nezhahq/nezha/model"
)

const autoCountryGroupNamePrefix = "[AUTO] Country: "

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
		var targetGroup model.ServerGroup
		err := tx.Where("user_id = ? AND name = ?", server.UserID, targetGroupName).First(&targetGroup).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			targetGroup = model.ServerGroup{
				Common: model.Common{
					UserID: server.UserID,
				},
				Name: targetGroupName,
			}
			if err := tx.Create(&targetGroup).Error; err != nil {
				return err
			}
		}

		// Ensure the server belongs to the target auto country group.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.ServerGroupServer{
			Common: model.Common{
				UserID: server.UserID,
			},
			ServerGroupId: targetGroup.ID,
			ServerId:      server.ID,
		}).Error; err != nil {
			return err
		}

		var otherAutoGroupIDs []uint64
		if err := tx.Model(&model.ServerGroup{}).
			Where("user_id = ? AND name LIKE ? AND id <> ?", server.UserID, autoCountryGroupNamePrefix+"%", targetGroup.ID).
			Pluck("id", &otherAutoGroupIDs).Error; err != nil {
			return err
		}
		if len(otherAutoGroupIDs) == 0 {
			return nil
		}

		// Move server out of other auto country groups.
		if err := tx.Delete(&model.ServerGroupServer{}, "server_id = ? AND server_group_id in (?)", server.ID, otherAutoGroupIDs).Error; err != nil {
			return err
		}

		// Remove empty auto country groups after moving.
		var activeGroupIDs []uint64
		if err := tx.Model(&model.ServerGroupServer{}).
			Distinct("server_group_id").
			Where("server_group_id in (?)", otherAutoGroupIDs).
			Pluck("server_group_id", &activeGroupIDs).Error; err != nil {
			return err
		}

		active := make(map[uint64]struct{}, len(activeGroupIDs))
		for _, id := range activeGroupIDs {
			active[id] = struct{}{}
		}

		staleGroupIDs := make([]uint64, 0, len(otherAutoGroupIDs))
		for _, id := range otherAutoGroupIDs {
			if _, ok := active[id]; !ok {
				staleGroupIDs = append(staleGroupIDs, id)
			}
		}

		if len(staleGroupIDs) > 0 {
			if err := tx.Delete(&model.ServerGroup{}, "id in (?)", staleGroupIDs).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
