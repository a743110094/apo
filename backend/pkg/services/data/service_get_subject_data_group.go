// Copyright 2025 CloudDetail
// SPDX-License-Identifier: Apache-2.0

package data

import (
	core "github.com/CloudDetail/apo/backend/pkg/core"
	"github.com/CloudDetail/apo/backend/pkg/model"
	"github.com/CloudDetail/apo/backend/pkg/model/datagroup"
	"github.com/CloudDetail/apo/backend/pkg/model/request"
	"github.com/CloudDetail/apo/backend/pkg/model/response"
	"github.com/CloudDetail/apo/backend/pkg/repository/database"
	"github.com/CloudDetail/apo/backend/pkg/services/common"
)

func (s *service) GetSubjectDataGroup(ctx core.Context, req *request.GetSubjectDataGroupRequest) (response.GetSubjectDataGroupResponse, error) {
	if req.SubjectType == model.DATA_GROUP_SUB_TYP_TEAM {
		return s.dbRepo.GetSubjectDataGroupList(ctx, req.SubjectID, req.SubjectType)
	}

	dataGroups, err := s.dbRepo.GetDataGroupByUserID(ctx, req.SubjectID)
	if err != nil {
		return nil, err
	}

	var groupIDs []int64
	var fromUser []int64
	var fromTeam []int64
	for _, group := range dataGroups {
		groupIDs = append(groupIDs, group.GroupID)
		if group.Source == model.DATA_GROUP_SUB_TYP_USER {
			fromUser = append(fromUser, group.GroupID)
		} else {
			fromTeam = append(fromTeam, group.GroupID)
		}
	}

	fullGroup := common.DataGroupStorage.GetFullPermissionGroupWithSource(groupIDs, fromUser, fromTeam)
	return fullGroup, nil
}

func (s *service) getDefaultDataGroup(ctx core.Context, category string) (datagroup.DataGroup, error) {
	defaultGroup := datagroup.DataGroup{
		GroupName: "default",
		Source:    model.DATA_GROUP_SOURCE_DEFAULT,
	}

	datasource, err := s.GetDataSource(ctx)
	if err != nil {
		return defaultGroup, err
	}

	filteredSources := make([]model.Datasource, 0)
	for _, ds := range datasource.NamespaceList {
		if category == "" || ds.Category == category {
			filteredSources = append(filteredSources, ds)
		}
	}

	for _, ds := range datasource.ServiceList {
		if category == "" || ds.Category == category {
			filteredSources = append(filteredSources, ds)
		}
	}

	items := make([]database.DatasourceGroup, 0, len(filteredSources))
	for _, ds := range filteredSources {
		items = append(items, database.DatasourceGroup{
			Datasource: ds.Datasource,
			Type:       ds.Type,
			Category:   ds.Category,
		})
	}

	return defaultGroup, nil
}
