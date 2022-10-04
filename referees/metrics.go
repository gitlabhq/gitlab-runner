package referees

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
)

type MetricsReferee struct {
	prometheusAPI prometheusV1.API
	queries       []string
	queryInterval time.Duration
	selector      string
	logger        logrus.FieldLogger
}

//nolint:lll
type MetricsRefereeConfig struct {
	PrometheusAddress string   `toml:"prometheus_address,omitempty" json:"prometheus_address" description:"A host:port to a prometheus metrics server"`
	QueryInterval     int      `toml:"query_interval,omitempty" json:"query_interval" description:"Query interval (in seconds)"`
	Queries           []string `toml:"queries" json:"queries" description:"A list of metrics to query (in PromQL)"`
}

//go:generate mockery --name=MetricsExecutor --inpackage
type MetricsExecutor interface {
	GetMetricsSelector() string
}

func (mr *MetricsReferee) ArtifactBaseName() string {
	return "metrics_referee.json"
}

func (mr *MetricsReferee) ArtifactType() string {
	return "metrics_referee"
}

func (mr *MetricsReferee) ArtifactFormat() string {
	return "gzip"
}

func (mr *MetricsReferee) Execute(ctx context.Context, startTime, endTime time.Time) (*bytes.Reader, error) {
	// specify the range used for the PromQL query
	queryRange := prometheusV1.Range{
		Start: startTime.UTC(),
		End:   endTime.UTC(),
		Step:  mr.queryInterval,
	}

	metrics := make(map[string][]model.SamplePair)
	// use config file to pull metrics from prometheus range queries
	for _, metricQuery := range mr.queries {
		// break up query into name:query
		components := strings.Split(metricQuery, ":")
		if len(components) != 2 {
			err := fmt.Errorf("%q not in name:query format in metric queries", metricQuery)
			mr.logger.WithError(err).Error("Failed to parse metrics query")

			return nil, err
		}

		name := components[0]
		query := components[1]

		result := mr.queryMetrics(ctx, query, queryRange)
		if result == nil {
			continue
		}

		metrics[name] = result
	}

	// convert metrics sample pairs to JSON
	output, err := json.Marshal(metrics)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(output), nil
}

func (mr *MetricsReferee) queryMetrics(
	ctx context.Context,
	query string,
	queryRange prometheusV1.Range,
) []model.SamplePair {
	interval := fmt.Sprintf("%.0fs", mr.queryInterval.Seconds())

	query = strings.ReplaceAll(query, "{selector}", mr.selector)
	query = strings.ReplaceAll(query, "{interval}", interval)

	queryLogger := mr.logger.WithFields(logrus.Fields{
		"query": query,
		"start": queryRange.Start,
		"end":   queryRange.End,
	})

	queryLogger.Debug("Sending request to Prometheus API")
	// execute query over range
	result, _, err := mr.prometheusAPI.QueryRange(ctx, query, queryRange)
	if err != nil {
		queryLogger.WithError(err).Error("Failed to range query Prometheus")
		return nil
	}

	if result == nil {
		queryLogger.Error("Received nil range query result")
		return nil
	}

	// ensure matrix result
	matrix, ok := result.(model.Matrix)
	if !ok {
		queryLogger.
			WithField("result-type", reflect.TypeOf(result)).
			Info("Failed to type assert result into model.Matrix")
		return nil
	}

	// no results for range query
	if matrix.Len() == 0 {
		return nil
	}

	// save first result set values at metric
	return matrix[0].Values
}

func newMetricsReferee(executor interface{}, config *Config, log logrus.FieldLogger) Referee {
	logger := log.WithField("referee", "metrics")
	if config.Metrics == nil {
		return nil
	}

	// see if provider supports metrics refereeing
	refereed, ok := executor.(MetricsExecutor)
	if !ok {
		logger.Info("executor not supported")
		return nil
	}

	// create prometheus client from server address in config
	clientConfig := api.Config{Address: config.Metrics.PrometheusAddress}
	prometheusClient, err := api.NewClient(clientConfig)
	if err != nil {
		logger.WithError(err).Error("failed to create prometheus client")
		return nil
	}

	prometheusAPI := prometheusV1.NewAPI(prometheusClient)

	return &MetricsReferee{
		prometheusAPI: prometheusAPI,
		queryInterval: time.Duration(config.Metrics.QueryInterval) * time.Second,
		queries:       config.Metrics.Queries,
		selector:      refereed.GetMetricsSelector(),
		logger:        logger,
	}
}
