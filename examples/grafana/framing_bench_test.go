// Originally from https://github.com/grafana/grafana/blob/c58542348d/pkg/tsdb/prometheus/querydata/framing_bench_test.go

package grafana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

// when memory-profiling this benchmark, these commands are recommended:
// - go test -benchmem -run=^$ -benchtime 1x -memprofile memprofile.out -memprofilerate 1 -bench ^BenchmarkExemplarJson$ github.com/grafana/grafana/pkg/tsdb/prometheus/buffered
// - go tool pprof -http=localhost:6061 memprofile.out
func BenchmarkExemplarJson(b *testing.B) {
	queryFileName := filepath.Join("../testdata", "exemplar.query.json")
	query, err := loadStoredQuery(queryFileName)
	require.NoError(b, err)

	responseFileName := filepath.Join("../testdata", "exemplar.result.json")

	// nolint:gosec
	// We can ignore the gosec G304 warning since this is a test file
	responseBytes, err := os.ReadFile(responseFileName)
	require.NoError(b, err)

	tCtx, err := setup(true)
	require.NoError(b, err)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		res := http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(responseBytes)),
		}
		tCtx.httpProvider.setResponse(&res)
		_, err := tCtx.queryData.Execute(context.Background(), query)
		require.NoError(b, err)
	}
}

// we store the prometheus query data in a json file, here is some minimal code
// to be able to read it back. unfortunately we cannot use the models.Query
// struct here, because it has `time.time` and `time.duration` fields that
// cannot be unmarshalled from JSON automatically.
type storedPrometheusQuery struct {
	RefId         string
	RangeQuery    bool
	ExemplarQuery bool
	Start         int64
	End           int64
	Step          int64
	Expr          string
	LegendFormat  string
}

func loadStoredQuery(fileName string) (*backend.QueryDataRequest, error) {
	//nolint:gosec
	bytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var sq storedPrometheusQuery

	err = json.Unmarshal(bytes, &sq)
	if err != nil {
		return nil, err
	}

	qm := QueryModel{
		RangeQuery:    sq.RangeQuery,
		ExemplarQuery: sq.ExemplarQuery,
		Expr:          sq.Expr,
		Interval:      fmt.Sprintf("%ds", sq.Step),
		IntervalMS:    sq.Step * 1000,
		LegendFormat:  sq.LegendFormat,
	}

	data, err := json.Marshal(&qm)
	if err != nil {
		return nil, err
	}

	return &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				TimeRange: backend.TimeRange{
					From: time.Unix(sq.Start, 0),
					To:   time.Unix(sq.End, 0),
				},
				RefID:    sq.RefId,
				Interval: time.Second * time.Duration(sq.Step),
				JSON:     json.RawMessage(data),
			},
		},
	}, nil
}
