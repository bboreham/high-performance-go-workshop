// Originally from https://github.com/grafana/grafana/blob/c58542348d/pkg/tsdb/prometheus/models/query.go

package grafana

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Internal interval and range variables
const (
	varInterval     = "$__interval"
	varIntervalMs   = "$__interval_ms"
	varRange        = "$__range"
	varRangeS       = "$__range_s"
	varRangeMs      = "$__range_ms"
	varRateInterval = "$__rate_interval"
)

// Internal interval and range variables with {} syntax
// Repetitive code, we should have functionality to unify these
const (
	varIntervalAlt     = "${__interval}"
	varIntervalMsAlt   = "${__interval_ms}"
	varRangeAlt        = "${__range}"
	varRangeSAlt       = "${__range_s}"
	varRangeMsAlt      = "${__range_ms}"
	varRateIntervalAlt = "${__rate_interval}"
)

var safeResolution = 11000

type QueryModel struct {
	Expr           string `json:"expr"`
	LegendFormat   string `json:"legendFormat"`
	Interval       string `json:"interval"`
	IntervalMS     int64  `json:"intervalMS"`
	StepMode       string `json:"stepMode"`
	RangeQuery     bool   `json:"range"`
	InstantQuery   bool   `json:"instant"`
	ExemplarQuery  bool   `json:"exemplar"`
	IntervalFactor int64  `json:"intervalFactor"`
	UtcOffsetSec   int64  `json:"utcOffsetSec"`
}

type TimeRange struct {
	Start time.Time
	End   time.Time
	Step  time.Duration
}

type Query struct {
	Expr          string
	Step          time.Duration
	LegendFormat  string
	Start         time.Time
	End           time.Time
	RefId         string
	InstantQuery  bool
	RangeQuery    bool
	ExemplarQuery bool
	UtcOffsetSec  int64
}

func (query *Query) TimeRange() TimeRange {
	return TimeRange{
		Step: query.Step,
		// Align query range to step. It rounds start and end down to a multiple of step.
		Start: AlignTimeRange(query.Start, query.Step, query.UtcOffsetSec),
		End:   AlignTimeRange(query.End, query.Step, query.UtcOffsetSec),
	}
}

func Parse(query backend.DataQuery, timeInterval string, intervalCalculator Calculator, fromAlert bool) (*Query, error) {
	model := &QueryModel{}
	if err := json.Unmarshal(query.JSON, model); err != nil {
		return nil, err
	}

	//Final interval value
	interval, err := calculatePrometheusInterval(model, timeInterval, query, intervalCalculator)
	if err != nil {
		return nil, err
	}

	// Interpolate variables in expr
	timeRange := query.TimeRange.To.Sub(query.TimeRange.From)
	expr := interpolateVariables(model, interval, timeRange, intervalCalculator, timeInterval)
	rangeQuery := model.RangeQuery
	if !model.InstantQuery && !model.RangeQuery {
		// In older dashboards, we were not setting range query param and !range && !instant was run as range query
		rangeQuery = true
	}

	// We never want to run exemplar query for alerting
	exemplarQuery := model.ExemplarQuery
	if fromAlert {
		exemplarQuery = false
	}

	return &Query{
		Expr:          expr,
		Step:          interval,
		LegendFormat:  model.LegendFormat,
		Start:         query.TimeRange.From,
		End:           query.TimeRange.To,
		RefId:         query.RefID,
		InstantQuery:  model.InstantQuery,
		RangeQuery:    rangeQuery,
		ExemplarQuery: exemplarQuery,
		UtcOffsetSec:  model.UtcOffsetSec,
	}, nil
}

func calculatePrometheusInterval(model *QueryModel, timeInterval string, query backend.DataQuery, intervalCalculator Calculator) (time.Duration, error) {
	queryInterval := model.Interval

	//If we are using variable for interval/step, we will replace it with calculated interval
	if isVariableInterval(queryInterval) {
		queryInterval = ""
	}

	minInterval, err := GetIntervalFrom(timeInterval, queryInterval, model.IntervalMS, 15*time.Second)
	if err != nil {
		return time.Duration(0), err
	}
	calculatedInterval := intervalCalculator.Calculate(query.TimeRange, minInterval, query.MaxDataPoints)
	safeInterval := intervalCalculator.CalculateSafeInterval(query.TimeRange, int64(safeResolution))

	adjustedInterval := safeInterval.Value
	if calculatedInterval.Value > safeInterval.Value {
		adjustedInterval = calculatedInterval.Value
	}

	if model.Interval == varRateInterval || model.Interval == varRateIntervalAlt {
		// Rate interval is final and is not affected by resolution
		return calculateRateInterval(adjustedInterval, timeInterval, intervalCalculator), nil
	} else {
		intervalFactor := model.IntervalFactor
		if intervalFactor == 0 {
			intervalFactor = 1
		}
		return time.Duration(int64(adjustedInterval) * intervalFactor), nil
	}
}

func calculateRateInterval(interval time.Duration, scrapeInterval string, intervalCalculator Calculator) time.Duration {
	scrape := scrapeInterval
	if scrape == "" {
		scrape = "15s"
	}

	scrapeIntervalDuration, err := ParseIntervalStringToTimeDuration(scrape)
	if err != nil {
		return time.Duration(0)
	}

	rateInterval := time.Duration(int64(math.Max(float64(interval+scrapeIntervalDuration), float64(4)*float64(scrapeIntervalDuration))))
	return rateInterval
}

func interpolateVariables(model *QueryModel, interval time.Duration, timeRange time.Duration, intervalCalculator Calculator, timeInterval string) string {
	expr := model.Expr
	rangeMs := timeRange.Milliseconds()
	rangeSRounded := int64(math.Round(float64(rangeMs) / 1000.0))

	var rateInterval time.Duration
	if model.Interval == varRateInterval || model.Interval == varRateIntervalAlt {
		rateInterval = interval
	} else {
		rateInterval = calculateRateInterval(interval, timeInterval, intervalCalculator)
	}

	expr = strings.ReplaceAll(expr, varIntervalMs, strconv.FormatInt(int64(interval/time.Millisecond), 10))
	expr = strings.ReplaceAll(expr, varInterval, FormatDuration(interval))
	expr = strings.ReplaceAll(expr, varRangeMs, strconv.FormatInt(rangeMs, 10))
	expr = strings.ReplaceAll(expr, varRangeS, strconv.FormatInt(rangeSRounded, 10))
	expr = strings.ReplaceAll(expr, varRange, strconv.FormatInt(rangeSRounded, 10)+"s")
	expr = strings.ReplaceAll(expr, varRateInterval, rateInterval.String())

	// Repetitive code, we should have functionality to unify these
	expr = strings.ReplaceAll(expr, varIntervalMsAlt, strconv.FormatInt(int64(interval/time.Millisecond), 10))
	expr = strings.ReplaceAll(expr, varIntervalAlt, FormatDuration(interval))
	expr = strings.ReplaceAll(expr, varRangeMsAlt, strconv.FormatInt(rangeMs, 10))
	expr = strings.ReplaceAll(expr, varRangeSAlt, strconv.FormatInt(rangeSRounded, 10))
	expr = strings.ReplaceAll(expr, varRangeAlt, strconv.FormatInt(rangeSRounded, 10)+"s")
	expr = strings.ReplaceAll(expr, varRateIntervalAlt, rateInterval.String())
	return expr
}

type ResultType string

const (
	ResultTypeMatrix   ResultType = "matrix"
	ResultTypeExemplar ResultType = "exemplar"
	ResultTypeVector   ResultType = "vector"
	ResultTypeUnknown  ResultType = ""
)

func ResultTypeFromFrame(frame *data.Frame) ResultType {
	if frame.Meta.Custom == nil {
		return ResultTypeUnknown
	}
	custom, ok := frame.Meta.Custom.(map[string]string)
	if !ok {
		return ResultTypeUnknown
	}

	rt, ok := custom["resultType"]
	if !ok {
		return ResultTypeUnknown
	}

	switch rt {
	case ResultTypeMatrix.String():
		return ResultTypeMatrix
	case ResultTypeExemplar.String():
		return ResultTypeExemplar
	case ResultTypeVector.String():
		return ResultTypeVector
	}

	return ResultTypeUnknown
}

func (r ResultType) String() string {
	return string(r)
}

func isVariableInterval(interval string) bool {
	if interval == varInterval || interval == varIntervalMs || interval == varRateInterval {
		return true
	}
	//Repetitive code, we should have functionality to unify these
	if interval == varIntervalAlt || interval == varIntervalMsAlt || interval == varRateIntervalAlt {
		return true
	}
	return false
}

func AlignTimeRange(t time.Time, step time.Duration, offset int64) time.Time {
	offsetNano := float64(offset * 1e9)
	stepNano := float64(step.Nanoseconds())
	return time.Unix(0, int64(math.Floor((float64(t.UnixNano())+offsetNano)/stepNano)*stepNano-offsetNano)).UTC()
}