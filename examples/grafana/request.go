// Originally from https://github.com/grafana/grafana/blob/c58542348d/pkg/tsdb/prometheus/querydata/request.go

package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

const legendFormatAuto = "__auto"

var legendFormatRegexp = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

type ExemplarEvent struct {
	Time   time.Time
	Value  float64
	Labels map[string]string
}

// QueryData handles querying but different from buffered package uses a custom client instead of default Go Prom
// client.
type QueryData struct {
	intervalCalculator Calculator
	client             *Client
	ID                 int64
	URL                string
	TimeInterval       string
	enableWideSeries   bool
}

func GetStringOptional(obj map[string]interface{}, key string) (string, error) {
	if untypedValue, ok := obj[key]; ok {
		if value, ok := untypedValue.(string); ok {
			return value, nil
		} else {
			err := fmt.Errorf("the field '%s' should be a string", key)
			return "", err
		}
	} else {
		// Value optional, not error
		return "", nil
	}
}

// GetJsonData just gets the json in easier to work with type. It's used on multiple places which isn't super effective
// but only when creating a client which should not happen often anyway.
func GetJsonData(settings backend.DataSourceInstanceSettings) (map[string]interface{}, error) {
	var jsonData map[string]interface{}
	err := json.Unmarshal(settings.JSONData, &jsonData)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JSONData: %w", err)
	}
	return jsonData, nil
}

func New(
	httpClient *http.Client,
	settings backend.DataSourceInstanceSettings,
) (*QueryData, error) {
	jsonData, err := GetJsonData(settings)
	if err != nil {
		return nil, err
	}
	httpMethod, _ := GetStringOptional(jsonData, "httpMethod")

	timeInterval, err := GetStringOptional(jsonData, "timeInterval")
	if err != nil {
		return nil, err
	}

	promClient := NewClient(httpClient, httpMethod, settings.URL)

	return &QueryData{
		intervalCalculator: NewCalculator(),
		client:             promClient,
		TimeInterval:       timeInterval,
		ID:                 settings.ID,
		URL:                settings.URL,
		enableWideSeries:   false,
	}, nil
}

func (s *QueryData) Execute(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	fromAlert := req.Headers["FromAlert"] == "true"
	result := backend.QueryDataResponse{
		Responses: backend.Responses{},
	}

	for _, q := range req.Queries {
		query, err := Parse(q, s.TimeInterval, s.intervalCalculator, fromAlert)
		if err != nil {
			return &result, err
		}
		r, err := s.fetch(ctx, s.client, query, req.Headers)
		if err != nil {
			return &result, err
		}
		if r == nil {
			//s.log.FromContext(ctx).Debug("Received nilresponse from runQuery", "query", query.Expr)
			continue
		}
		result.Responses[q.RefID] = *r
	}

	return &result, nil
}

func (s *QueryData) fetch(ctx context.Context, client *Client, q *Query, headers map[string]string) (*backend.DataResponse, error) {
	response := &backend.DataResponse{
		Frames: data.Frames{},
		Error:  nil,
	}

	if q.InstantQuery {
		res, err := s.instantQuery(ctx, client, q, headers)
		if err != nil {
			return nil, err
		}
		response.Error = res.Error
		response.Frames = res.Frames
	}

	if q.RangeQuery {
		res, err := s.rangeQuery(ctx, client, q, headers)
		if err != nil {
			return nil, err
		}
		if res.Error != nil {
			if response.Error == nil {
				response.Error = res.Error
			} else {
				response.Error = fmt.Errorf("%v %w", response.Error, res.Error) // lovely
			}
		}
		response.Frames = append(response.Frames, res.Frames...)
	}

	if q.ExemplarQuery {
		res, err := s.exemplarQuery(ctx, client, q, headers)
		if err != nil {
			// If exemplar query returns error, we want to only log it and
			// continue with other results processing
			//logger.Error("Exemplar query failed", "query", q.Expr, "err", err)
		}
		if res != nil {
			response.Frames = append(response.Frames, res.Frames...)
		}
	}

	return response, nil
}

func (s *QueryData) rangeQuery(ctx context.Context, c *Client, q *Query, headers map[string]string) (*backend.DataResponse, error) {
	res, err := c.QueryRange(ctx, q, sdkHeaderToHttpHeader(headers))
	if err != nil {
		return nil, err
	}
	return s.parseResponse(ctx, q, res)
}

func (s *QueryData) instantQuery(ctx context.Context, c *Client, q *Query, headers map[string]string) (*backend.DataResponse, error) {
	res, err := c.QueryInstant(ctx, q, sdkHeaderToHttpHeader(headers))
	if err != nil {
		return nil, err
	}
	return s.parseResponse(ctx, q, res)
}

func (s *QueryData) exemplarQuery(ctx context.Context, c *Client, q *Query, headers map[string]string) (*backend.DataResponse, error) {
	res, err := c.QueryExemplars(ctx, q, sdkHeaderToHttpHeader(headers))
	if err != nil {
		return nil, err
	}
	return s.parseResponse(ctx, q, res)
}

func sdkHeaderToHttpHeader(headers map[string]string) http.Header {
	httpHeader := make(http.Header)
	for key, val := range headers {
		httpHeader[key] = []string{val}
	}
	return httpHeader
}
