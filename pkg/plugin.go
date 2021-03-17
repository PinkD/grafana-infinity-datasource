package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

type queryModel struct{}

func newDatasource() datasource.ServeOpts {
	im := datasource.NewInstanceManager(newDataSourceInstance)
	ds := &InfinityDatasource{
		im: im,
	}
	return datasource.ServeOpts{
		QueryDataHandler:   ds,
		CheckHealthHandler: ds,
	}
}

// InfinityDatasource is an example datasource used to scaffold
type InfinityDatasource struct {
	im instancemgmt.InstanceManager
}

// CheckHealth handles health checks sent from Grafana to the plugin.
func (td *InfinityDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source healthcheck not implemented",
	}, nil
}

// QueryData handles multiple queries and returns multiple responses.
func (td *InfinityDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (response *backend.QueryDataResponse, err error) {
	for _, q := range req.Queries {
		res := td.query(ctx, q)
		response.Responses[q.RefID] = res
	}
	return response, nil
}

func (td *InfinityDatasource) query(ctx context.Context, query backend.DataQuery) (response backend.DataResponse) {
	var qm queryModel
	response.Error = json.Unmarshal(query.JSON, &qm)
	if response.Error != nil {
		return response
	}
	frame := data.NewFrame("response")
	response.Frames = append(response.Frames, frame)
	return response
}

type instanceSettings struct {
	httpClient *http.Client
}

func (s *instanceSettings) Dispose() {}

func newDataSourceInstance(setting backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &instanceSettings{
		httpClient: &http.Client{},
	}, nil
}
