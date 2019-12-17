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
	labelName     string
	labelValue    string
	logger        *logrus.Entry
}

type MetricsRefereeConfig struct {
	PrometheusAddress string   `toml:"prometheus_address,omitempty" json:"prometheus_address" description:"A host:port to a prometheus metrics server"`
	QueryInterval     int      `toml:"query_interval,omitempty" json:"query_interval" description:"Query interval (in seconds)"`
	Queries           []string `toml:"queries" json:"queries" description:"A list of metrics to query (in PromQL)"`
}

type MetricsExecutor interface {
	GetMetricsLabelName() string
	GetMetricsLabelValue() string
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

func (mr *MetricsReferee) Execute(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
) (*bytes.Reader, error) {
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
			err := fmt.Errorf("%s not in name:query format in metric queries", metricQuery)
			mr.logger.WithError(err).Error("Failed to parse metrics query")
			return nil, err
		}

		name := components[0]
		query := components[1]
		selector := fmt.Sprintf("%s=\"%s\"", mr.labelName, mr.labelValue)
		interval := fmt.Sprintf("%.0fs", mr.queryInterval.Seconds())
		query = strings.Replace(query, "{selector}", selector, -1)
		query = strings.Replace(query, "{interval}", interval, -1)
		queryLogger := mr.logger.WithFields(logrus.Fields{
			"query": query,
			"start": startTime.UTC(),
			"end":   endTime.UTC(),
		})

		queryLogger.Debug("Sending request to Prometheus API")
		// execute query over range
		result, _, err := mr.prometheusAPI.QueryRange(ctx, query, queryRange)
		if err != nil {
			queryLogger.WithError(err).Error("Failed to range query Prometheus")
			continue
		}

		if result == nil {
			queryLogger.Error("Received nil range query result")
			continue
		}

		// ensure matrix result
		matrix, ok := result.(model.Matrix)
		if !ok {
			queryLogger.WithField("type", reflect.TypeOf(result)).Info("Failed to type assert result into model.Matrix")
			continue
		}

		// no results for range query
		if matrix.Len() == 0 {
			continue
		}

		// save first result set values at metric
		metrics[name] = matrix[0].Values
	}

	// convert metrics sample pairs to JSON
	output, _ := json.Marshal(metrics)
	return bytes.NewReader(output), nil
}

func CreateMetricsReferee(executor interface{}, config *Config, log *logrus.Entry) *MetricsReferee {
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
		labelName:     refereed.GetMetricsLabelName(),
		labelValue:    refereed.GetMetricsLabelValue(),
		logger:        logger,
	}
}
