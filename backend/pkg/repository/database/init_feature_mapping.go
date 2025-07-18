// Copyright 2024 CloudDetail
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"errors"
	"net/http"
	"strings"

	core "github.com/CloudDetail/apo/backend/pkg/core"
	"github.com/CloudDetail/apo/backend/pkg/model"
	"github.com/CloudDetail/apo/backend/pkg/model/profile"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func isValidMenuItem(menuItem string) bool {
	for _, mapping := range validMenuItemMappings {
		if menuItem == mapping.MenuItem.Key {
			return true
		}
	}

	return false
}

func (repo *daoRepo) initFeatureMenuItems(ctx core.Context) error {
	featureMenuMapping := []struct {
		FeatureName string
		MenuKey     string
	}{
		{"服务概览", "service"},
		{"故障现场日志", "faultSite"},
		{"全量日志", "full"},
		{"故障现场链路", "faultSiteTrace"},
		{"全量链路", "fullTrace"},
		{"全局资源大盘", "system"},
		{"应用基础设施大盘", "basic"},
		{"应用指标大盘", "application"},
		{"中间件大盘", "middleware"},
		{"告警规则", "alertsRule"},
		{"告警通知", "alertsNotify"},
		{"告警事件", "alertEvents"},
		{"数据接入", "dataIntegration"},
		{"告警接入", "alertsIntegration"},
		{"工作流", "workflows"},
		{"配置中心", "config"},
		{"用户管理", "userManage"},
		{"菜单管理", "menuManage"},
		{"团队管理", "team"},
		{"角色管理", "role"},
	}

	return repo.GetContextDB(ctx).Transaction(func(tx *gorm.DB) error {
		var featureIDs, menuItemIDs []int
		if err := tx.Model(&profile.Feature{}).Select("feature_id").Find(&featureIDs).Error; err != nil {
			return err
		}

		if err := tx.Model(&MenuItem{}).Select("item_id").Where("router_id != 0").Find(&menuItemIDs).Error; err != nil {
			return err
		}
		// delete mapping whose feature or menu has been already deleted
		if err := tx.Model(&FeatureMapping{}).
			Where("feature_id not in ? OR (mapped_id NOT IN ? AND mapped_type = ?)", featureIDs, menuItemIDs, model.MAPPED_TYP_MENU).
			Delete(nil).Error; err != nil {
			return err
		}

		for _, mapping := range featureMenuMapping {
			if !isValidFeature(mapping.FeatureName) {
				continue
			}

			if !isValidMenuItem(mapping.MenuKey) {
				continue
			}

			var feature profile.Feature
			if err := tx.Where("feature_name = ?", mapping.FeatureName).First(&feature).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			if feature.FeatureID <= 0 {
				continue
			}

			var menuItem MenuItem
			if err := tx.Where(`"key" = ?`, mapping.MenuKey).First(&menuItem).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			if menuItem.ItemID <= 0 {
				continue
			}

			var count int64
			if err := tx.Model(&FeatureMapping{}).
				Where("feature_id = ? AND mapped_id = ? AND mapped_type = ?", feature.FeatureID, menuItem.ItemID, model.MAPPED_TYP_MENU).
				Count(&count).Error; err != nil {
				return err
			}

			if count == 0 {
				featureMenuItem := FeatureMapping{
					FeatureID:  feature.FeatureID,
					MappedID:   menuItem.ItemID,
					MappedType: model.MAPPED_TYP_MENU,
				}
				if err := tx.Create(&featureMenuItem).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// TODO add mapping of feature and api

func (repo *daoRepo) initFeatureRouter(ctx core.Context) error {
	featureRoutes := map[string][]string{
		"服务概览": {"/service/info", "/service/overview"},
		"数据接入": {"/integration/data/settings", "/data/ingestion"},
		"个人中心": {"/user", "/profile", "/account"},
		"告警事件": {"/alerts/events/detail/:alertID/:eventID"},
	}

	return repo.GetContextDB(ctx).Transaction(func(tx *gorm.DB) error {
		var featureIDs, routerIDs []int
		if err := tx.Model(&profile.Feature{}).Select("feature_id").Find(&featureIDs).Error; err != nil {
			return err
		}

		if err := tx.Model(&Router{}).Select("router_id").Find(&routerIDs).Error; err != nil {
			return err
		}

		if err := tx.Model(&FeatureMapping{}).
			Where("feature_id not in ? OR (mapped_id NOT IN ? AND mapped_type = ?)",
				featureIDs,
				routerIDs,
				model.MAPPED_TYP_ROUTER).
			Delete(nil).Error; err != nil {
			return err
		}

		for featureName, routerPaths := range featureRoutes {
			var feature profile.Feature
			if err := tx.Where("feature_name = ?", featureName).First(&feature).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			for _, routerPath := range routerPaths {
				var router Router
				if err := tx.Where(`"router_to" = ?`, routerPath).First(&router).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}

				var count int64
				if err := tx.Model(&FeatureMapping{}).
					Where("feature_id = ? AND mapped_id = ? AND mapped_type = ?",
						feature.FeatureID,
						router.RouterID,
						model.MAPPED_TYP_ROUTER).
					Count(&count).Error; err != nil {
					return err
				}

				if count == 0 {
					mapping := FeatureMapping{
						FeatureID:  feature.FeatureID,
						MappedID:   router.RouterID,
						MappedType: model.MAPPED_TYP_ROUTER,
					}
					if err := tx.Create(&mapping).Error; err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

func isValidFeature(featureName string) bool {
	for _, validFeature := range validFeatures {
		if featureName == validFeature.FeatureName {
			return true
		}
	}

	return false
}

func isValidMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	default:
		return false
	}
}

func isValidPath(path string) bool {
	return len(path) > 0 && path[0] == '/' && !strings.ContainsAny(path, ";&'=")
}

func (repo *daoRepo) initFeatureAPI(ctx core.Context) error {
	featureAPI := map[string][]API{}
	viper.SetConfigType("yaml")
	viper.SetConfigFile("./sqlscripts/feature_api.yml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	if err := viper.Unmarshal(&featureAPI); err != nil {
		return err
	}

	return repo.GetContextDB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&FeatureMapping{}); err != nil {
			return err
		}

		var featureIDs, apiIDs []int
		if err := tx.Model(&profile.Feature{}).Select("feature_id").Find(&featureIDs).Error; err != nil {
			return err
		}

		if err := tx.Model(&API{}).Select("id").Find(&apiIDs).Error; err != nil {
			return err
		}

		// delete mapping whose feature or api has been already deleted
		if err := tx.Model(&FeatureMapping{}).
			Where("feature_id not in ? OR (mapped_id NOT IN ? AND mapped_type = ?)", featureIDs, apiIDs, model.MAPPED_TYP_API).
			Delete(nil).Error; err != nil {
			return err
		}

		for featureName, apis := range featureAPI {
			var feature profile.Feature
			if !isValidFeature(featureName) {
				continue
			}

			if err := tx.Where("feature_name = ?", featureName).First(&feature).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}

			validApis := make([]API, 0, len(apis))
			for _, api := range apis {
				if isValidMethod(api.Method) && isValidPath(api.Path) {
					validApis = append(validApis, api)
				}
			}

			for _, api := range validApis {
				var apiRecord API
				if err := tx.Where("path = ? AND method = ?", api.Path, api.Method).First(&apiRecord).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}

				var count int64
				if err := tx.Model(&FeatureMapping{}).
					Where("feature_id = ? AND mapped_id = ? AND mapped_type = ?", feature.FeatureID, apiRecord.ID, model.MAPPED_TYP_API).
					Count(&count).Error; err != nil {
					return err
				}

				if count == 0 {
					featureAPI := FeatureMapping{
						FeatureID:  feature.FeatureID,
						MappedID:   apiRecord.ID,
						MappedType: model.MAPPED_TYP_API,
					}
					if err := tx.Create(&featureAPI).Error; err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}
