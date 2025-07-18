// Copyright 2025 CloudDetail
// SPDX-License-Identifier: Apache-2.0

package dataplane

import (
	"time"

	core "github.com/CloudDetail/apo/backend/pkg/core"
	"github.com/CloudDetail/apo/backend/pkg/util"

	"github.com/CloudDetail/apo/backend/pkg/model/request"
	"github.com/CloudDetail/apo/backend/pkg/model/response"
	"github.com/CloudDetail/apo/backend/pkg/repository/clickhouse"
	"github.com/CloudDetail/apo/backend/pkg/repository/prometheus"
)

type ServiceEndpointMap = prometheus.MetricGroupMap[prometheus.EndpointKey, *prometheus.ServiceEndpointMetrics]

func (s *service) GetServiceEndpointRedCharts(ctx core.Context, req *request.QueryServiceRedChartsRequest) *response.QueryServiceRedChartsResponse {
	stepMicros := getStepMicros(req.EndTime - req.StartTime)
	startBucket := req.StartTime / stepMicros
	endBucket := req.EndTime / stepMicros
	results := make([]*response.QueryChartResult, 0)

	bucketMetrics, err := s.chRepo.QueryGroupServiceRedMetrics(ctx, req.StartTime, req.EndTime, req.Cluster, req.ServiceName, req.Endpoint, stepMicros)
	if err != nil {
		return &response.QueryServiceRedChartsResponse{
			Msg: "query redmetrics: " + err.Error(),
		}
	}
	if len(bucketMetrics) == 0 {
		return &response.QueryServiceRedChartsResponse{
			Results: results,
		}
	}

	bucketMap := make(map[int64]*clickhouse.BucketRedMetric)
	for _, bucketMetric := range bucketMetrics {
		bucketMap[int64(bucketMetric.TimeBucket)] = &bucketMetric
	}

	countChart := make(map[int64]float64)
	errorChart := make(map[int64]float64)
	latencyChart := make(map[int64]float64)
	groupMetrics := &clickhouse.GroupRedMetric{
		Duration: req.EndTime - req.StartTime,
	}
	for i := startBucket; i <= endBucket; i++ {
		bucketMetric, ok := bucketMap[i]
		if !ok {
			// Fill Zero
			countChart[i*stepMicros] = 0.0
			errorChart[i*stepMicros] = 0.0
			latencyChart[i*stepMicros] = 0.0
		} else {
			if bucketMetric.TotalCount == 0 {
				countChart[i*stepMicros] = 0.0
				errorChart[i*stepMicros] = 0.0
				latencyChart[i*stepMicros] = 0.0
			} else {
				countChart[i*stepMicros] = float64(int64(bucketMetric.TotalCount) * 60_000_000 / stepMicros)
				errorChart[i*stepMicros] = float64(100 * bucketMetric.TotalError / bucketMetric.TotalCount)
				latencyChart[i*stepMicros] = float64(bucketMetric.TotalDuration / uint64(bucketMetric.TotalCount))

				groupMetrics.TotalCount += bucketMetric.TotalCount
				groupMetrics.TotalError += bucketMetric.TotalError
				groupMetrics.TotalDuration += bucketMetric.TotalDuration
			}
		}
	}

	dodMetrics, err := s.chRepo.QueryGroupServiceRedMetricValue(ctx, req.StartTime-24*int64(time.Hour)/1000, req.StartTime, req.Cluster, req.ServiceName, req.Endpoint)
	if err != nil {
		return &response.QueryServiceRedChartsResponse{
			Msg: "query dod metric failed: " + err.Error(),
		}
	}
	wowMetrics, err := s.chRepo.QueryGroupServiceRedMetricValue(ctx, req.StartTime-7*24*int64(time.Hour)/1000, req.StartTime, req.Cluster, req.ServiceName, req.Endpoint)
	if err != nil {
		return &response.QueryServiceRedChartsResponse{
			Msg: "query wow metric failed: " + err.Error(),
		}
	}
	avgLatency := groupMetrics.GetAvgLatency()
	results = append(results, &response.QueryChartResult{
		Title: "平均响应时间",
		Unit:  "ms",
		Timeseries: []*response.Timeseries{
			{
				Legend:       req.ServiceName,
				LegendFormat: "",
				Labels: map[string]string{
					"service":  req.ServiceName,
					"endpoint": req.Endpoint,
				},
				Chart: response.TempChartObject{
					ChartData: latencyChart,
					Ratio: response.Ratio{
						DayOverDay:  util.PtrFloat64(GetRatio(avgLatency, dodMetrics.GetAvgLatency())),
						WeekOverDay: util.PtrFloat64(GetRatio(avgLatency, wowMetrics.GetAvgLatency())),
					},
					Value: util.PtrFloat64(avgLatency),
				},
			},
		},
	})
	errorRate := groupMetrics.GetErrorRate()
	results = append(results, &response.QueryChartResult{
		Title: "错误率",
		Unit:  "%",
		Timeseries: []*response.Timeseries{
			{
				Legend:       req.ServiceName,
				LegendFormat: "",
				Labels: map[string]string{
					"service":  req.ServiceName,
					"endpoint": req.Endpoint,
				},
				Chart: response.TempChartObject{
					ChartData: errorChart,
					Ratio: response.Ratio{
						DayOverDay:  util.PtrFloat64(GetRatio(errorRate, dodMetrics.GetErrorRate())),
						WeekOverDay: util.PtrFloat64(GetRatio(errorRate, wowMetrics.GetErrorRate())),
					},
					Value: util.PtrFloat64(errorRate),
				},
			},
		},
	})
	tpm := groupMetrics.GetTpm()
	results = append(results, &response.QueryChartResult{
		Title: "吞吐量",
		Unit:  "次/分",
		Timeseries: []*response.Timeseries{
			{
				Legend:       req.ServiceName,
				LegendFormat: "",
				Labels: map[string]string{
					"service":  req.ServiceName,
					"endpoint": req.Endpoint,
				},
				Chart: response.TempChartObject{
					ChartData: countChart,
					Ratio: response.Ratio{
						DayOverDay:  util.PtrFloat64(GetRatio(tpm, dodMetrics.GetTpm())),
						WeekOverDay: util.PtrFloat64(GetRatio(tpm, wowMetrics.GetTpm())),
					},
					Value: util.PtrFloat64(tpm),
				},
			},
		},
	})
	return &response.QueryServiceRedChartsResponse{
		Results: results,
	}
}

func (s *service) getServiceEndpointRedChartsByApo(ctx core.Context, req *request.QueryServiceRedChartsRequest) *response.QueryServiceRedChartsResponse {
	endpointKey := prometheus.EndpointKey{
		SvcName:    req.ServiceName,
		ContentKey: req.Endpoint,
	}
	endpointMap := &ServiceEndpointMap{
		MetricGroupList: []*prometheus.ServiceEndpointMetrics{},
		MetricGroupMap: map[prometheus.EndpointKey]*prometheus.ServiceEndpointMetrics{
			endpointKey: {
				EndpointKey: endpointKey,
			},
		},
	}
	startTime := time.Unix(req.StartTime/1000000, 0)
	endTime := time.Unix(req.EndTime/1000000, 0)
	granularity := prometheus.EndpointGranularity

	filter := prometheus.NewFilter().
		Equal(prometheus.ServiceNameKey, req.ServiceName).
		Equal(prometheus.ContentKeyKey, req.Endpoint)

		// Chart data
	stepMicros := getStepMicros(req.EndTime - req.StartTime)
	latencyRes, latencyErr := s.promRepo.QueryRangeMetricsWithPQLFilter(ctx,
		prometheus.PQLAvgLatencyWithPQLFilter,
		req.StartTime,
		req.EndTime,
		stepMicros,
		granularity,
		filter,
	)
	if latencyErr != nil {
		return &response.QueryServiceRedChartsResponse{
			Msg: "query avg metrics failed: " + latencyErr.Error(),
		}
	}

	if len(latencyRes) == 0 {
		return &response.QueryServiceRedChartsResponse{
			Results: []*response.QueryChartResult{},
		}
	}
	mergeEndpointChartMetrics(endpointMap, latencyRes, metricLatencyData)

	errorRes, rateErr := s.promRepo.QueryRangeMetricsWithPQLFilter(ctx,
		prometheus.PQLAvgErrorRateWithPQLFilter,
		req.StartTime,
		req.EndTime,
		stepMicros,
		granularity,
		filter,
	)
	if rateErr != nil {
		return &response.QueryServiceRedChartsResponse{
			Msg: "query errorRate metrics failed: " + rateErr.Error(),
		}
	}
	mergeEndpointChartMetrics(endpointMap, errorRes, metricErrorData)

	tpmRes, tmpErr := s.promRepo.QueryRangeMetricsWithPQLFilter(ctx,
		prometheus.PQLAvgTPSWithPQLFilter,
		req.StartTime,
		req.EndTime,
		stepMicros,
		granularity,
		filter,
	)
	if tmpErr != nil {
		return &response.QueryServiceRedChartsResponse{
			Msg: "query tps metrics failed: " + tmpErr.Error(),
		}
	}
	mergeEndpointChartMetrics(endpointMap, tpmRes, metricTPMData)

	// Metric Value
	s.promRepo.FillMetric(ctx, endpointMap, prometheus.AVG, startTime, endTime, filter, granularity)
	s.promRepo.FillMetric(ctx, endpointMap, prometheus.DOD, startTime, endTime, filter, granularity)
	s.promRepo.FillMetric(ctx, endpointMap, prometheus.WOW, startTime, endTime, filter, granularity)

	results := make([]*response.QueryChartResult, 0)
	for _, endpointMetric := range endpointMap.MetricGroupMap {
		latencyTempChartObject := response.TempChartObject{
			Ratio: response.Ratio{
				DayOverDay:  endpointMetric.REDMetrics.DOD.Latency,
				WeekOverDay: endpointMetric.REDMetrics.WOW.Latency,
			},
			Value: endpointMetric.REDMetrics.Avg.Latency,
		}
		if endpointMetric.LatencyData != nil {
			latencyTempChartObject.ChartData = DataToChart(endpointMetric.LatencyData)
		} else {
			latencyTempChartObject.ChartData = FillChart(startTime, endTime, stepMicros)
		}
		results = append(results, &response.QueryChartResult{
			Title: "平均响应时间",
			Unit:  "ms",
			Timeseries: []*response.Timeseries{
				{
					Legend:       req.ServiceName,
					LegendFormat: "",
					Labels: map[string]string{
						"source":   "apo",
						"service":  req.ServiceName,
						"endpoint": req.Endpoint,
					},
					Chart: latencyTempChartObject,
				},
			},
		})

		errorTempChartObject := response.TempChartObject{
			Ratio: response.Ratio{
				DayOverDay:  endpointMetric.REDMetrics.DOD.ErrorRate,
				WeekOverDay: endpointMetric.REDMetrics.WOW.ErrorRate,
			},
			Value: endpointMetric.REDMetrics.Avg.ErrorRate,
		}
		if errorTempChartObject.Value == nil {
			zero := new(float64)
			errorTempChartObject.Value = zero
		}
		if endpointMetric.ErrorRateData != nil {
			errorTempChartObject.ChartData = DataToChart(endpointMetric.ErrorRateData)
		} else {
			errorTempChartObject.ChartData = FillChart(startTime, endTime, stepMicros)
		}
		results = append(results, &response.QueryChartResult{
			Title: "错误率",
			Unit:  "%",
			Timeseries: []*response.Timeseries{
				{
					Legend:       req.ServiceName,
					LegendFormat: "",
					Labels: map[string]string{
						"source":   "apo",
						"service":  req.ServiceName,
						"endpoint": req.Endpoint,
					},
					Chart: errorTempChartObject,
				},
			},
		})

		// construct tps return value
		tpmTempChartObject := response.TempChartObject{
			Ratio: response.Ratio{
				DayOverDay:  endpointMetric.REDMetrics.DOD.TPM,
				WeekOverDay: endpointMetric.REDMetrics.WOW.TPM,
			},
			Value: endpointMetric.REDMetrics.Avg.TPM,
		}
		if endpointMetric.TPMData != nil {
			tpmTempChartObject.ChartData = DataToChart(endpointMetric.TPMData)
		} else {
			tpmTempChartObject.ChartData = FillChart(startTime, endTime, stepMicros)
		}
		results = append(results, &response.QueryChartResult{
			Title: "吞吐量",
			Unit:  "次/分",
			Timeseries: []*response.Timeseries{
				{
					Legend:       req.ServiceName,
					LegendFormat: "",
					Labels: map[string]string{
						"source":   "apo",
						"service":  req.ServiceName,
						"endpoint": req.Endpoint,
					},
					Chart: tpmTempChartObject,
				},
			},
		})
	}

	return &response.QueryServiceRedChartsResponse{
		Results: results,
	}
}

func mergeEndpointChartMetrics(endpointMap *ServiceEndpointMap, results []prometheus.MetricResult, metricName string) {
	for _, res := range results {
		var kType prometheus.EndpointKey
		key := kType.ConvertFromLabels(res.Metric).(prometheus.EndpointKey)

		serviceEndpoint, ok := endpointMap.MetricGroupMap[key]
		if !ok {
			continue
		}
		switch metricName {
		case metricLatencyData:
			for i := range res.Values {
				res.Values[i].Value /= 1e3
			}
			serviceEndpoint.LatencyData = res.Values
		case metricErrorData:
			for i := range res.Values {
				res.Values[i].Value *= 100
			}
			serviceEndpoint.ErrorRateData = res.Values
		case metricTPMData:
			for i := range res.Values {
				res.Values[i].Value *= 60
			}
			serviceEndpoint.TPMData = res.Values
		}
	}
}
