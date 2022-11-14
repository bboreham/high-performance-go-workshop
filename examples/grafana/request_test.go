// Originally from https://github.com/grafana/grafana/blob/c58542348d/pkg/tsdb/prometheus/querydata/request_test.go

package grafana

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	sdkhttpclient "github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

type testContext struct {
	httpProvider *fakeHttpClientProvider
	queryData    *QueryData
}

func setup(wideFrames bool) (*testContext, error) {
	httpProvider := &fakeHttpClientProvider{
		opts: sdkhttpclient.Options{
			Timeouts: &sdkhttpclient.DefaultTimeoutOptions,
		},
		res: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
		},
	}
	settings := backend.DataSourceInstanceSettings{
		URL:      "http://localhost:9090",
		JSONData: json.RawMessage(`{"timeInterval": "15s"}`),
	}

	opts, err := settings.HTTPClientOptions()
	if err != nil {
		return nil, err
	}

	httpClient, err := httpProvider.New(opts)
	if err != nil {
		return nil, err
	}

	queryData, _ := New(httpClient, settings)

	return &testContext{
		httpProvider: httpProvider,
		queryData:    queryData,
	}, nil
}

type httpclientProvider interface {
	// New creates a new http.Client given provided options.
	New(opts ...sdkhttpclient.Options) (*http.Client, error)

	// GetTransport creates a new http.RoundTripper given provided options.
	GetTransport(opts ...sdkhttpclient.Options) (http.RoundTripper, error)

	// GetTLSConfig creates a new tls.Config given provided options.
	GetTLSConfig(opts ...sdkhttpclient.Options) (*tls.Config, error)
}

type fakeHttpClientProvider struct {
	httpclientProvider
	opts sdkhttpclient.Options
	res  *http.Response
}

func (p *fakeHttpClientProvider) New(opts ...sdkhttpclient.Options) (*http.Client, error) {
	p.opts = opts[0]
	c, err := sdkhttpclient.New(opts[0])
	if err != nil {
		return nil, err
	}
	c.Transport = p
	return c, nil
}

func (p *fakeHttpClientProvider) GetTransport(opts ...sdkhttpclient.Options) (http.RoundTripper, error) {
	p.opts = opts[0]
	return http.DefaultTransport, nil
}

func (p *fakeHttpClientProvider) setResponse(res *http.Response) {
	p.res = res
}

func (p *fakeHttpClientProvider) RoundTrip(req *http.Request) (*http.Response, error) {
	return p.res, nil
}
