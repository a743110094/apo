// Copyright 2024 CloudDetail
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"net/http"

	"github.com/CloudDetail/apo/backend/pkg/code"
	"github.com/CloudDetail/apo/backend/pkg/core"
	"github.com/CloudDetail/apo/backend/pkg/model/integration/alert"
	"github.com/CloudDetail/apo/backend/pkg/model/request"
	"github.com/CloudDetail/apo/backend/pkg/model/response"
)

// GetAlertEvents get alarm events
// @Summary get alarm events
// @Description get alarm events
// @Tags API.service
// @Accept application/json
// @Produce json
// @Param startTime query int64 true "query start time"
// @Param endTime query int64 true "query end time"
// @Param service query string false "Query service name"
// @Param services query []string false "query service list" collectionFormat(multi)
// @Param source query string false "Query the alarm source"
// @Param group query string false "Query alarm type"
// @Param name query string false "Query alarm name"
// @Param id query string false "Query alarm ID"
// @Param status query string false "Query alarm status"
// @Param currentPage query int false "Paging parameter, current number of pages"
// @Param pageSize query int false "Pagination parameter, quantity per page"
// @Param Authorization header string false "Bearer accessToken"
// @Success 200 {object} response.GetAlertEventsResponse
// @Failure 400 {object} code.Failure
// @Router /api/service/alert/events [post]
func (h *handler) GetAlertEvents() core.HandlerFunc {
	return func(c core.Context) {
		req := new(request.GetAlertEventsRequest)
		if err := c.ShouldBind(req); err != nil {
			c.AbortWithError(
				http.StatusBadRequest,
				code.ParamBindError,
				err,
			)
			return
		}

		// TODO Compatible with old APIs
		{
			if len(req.Service) > 0 {
				req.Services = append(req.Services, req.Service)
			}
			if len(req.Endpoint) > 0 {
				req.Endpoints = append(req.Endpoints, req.Endpoint)
			}
		}

		resp, err := h.serviceInfoService.GetAlertEvents(c, req)
		if err != nil {
			c.AbortWithError(
				http.StatusBadRequest,
				code.GetAlertEventsError,
				err,
			)
			return
		}
		if resp == nil {
			resp = &response.GetAlertEventsResponse{
				TotalCount: 0,
				EventList:  []alert.AlertEvent{},
			}
		}
		c.Payload(resp)
	}
}
