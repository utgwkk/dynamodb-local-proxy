package handler

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/cockroachdb/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/itchyny/gojq"
	sloghttp "github.com/samber/slog-http"
	"github.com/thinkgos/httpcurl"
	"github.com/utgwkk/dynamodb-local-proxy/internal/util"
	"github.com/utgwkk/slogerr"
)

//go:embed fill_warm_throughput.jq
var gojqQueryStr string

var gojqQuery = util.Must(gojq.Compile(
	util.Must(gojq.Parse(gojqQueryStr)),
))

type Handler struct {
	DynamoDBLocalAddr string

	proxy *httputil.ReverseProxy
}

func New(
	dynamodbLocalAddr string,
	httpClient *http.Client,
) *Handler {
	h := &Handler{
		DynamoDBLocalAddr: dynamodbLocalAddr,
	}

	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	h.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = dynamodbLocalAddr
			req.Header.Set("User-Agent", "github.com/utgwkk/dynamodb-local-proxy")
		},
		Transport: &loggingTransport{rt: transport},
		ModifyResponse: func(resp *http.Response) error {
			if isDescribeTableRequest(resp.Request) && resp.StatusCode == http.StatusOK {
				return rewriteDescribeTableResponse(resp)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.ErrorContext(r.Context(), "proxy error", slogerr.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
		},
	}

	return h
}

func generateRequestId() string {
	id, err := uuid.NewV6()
	if err != nil {
		slog.Error("generateRequestId failed", slogerr.Error(err))
		return ""
	}
	return id.String()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if requestId := generateRequestId(); requestId != "" {
		r = r.WithContext(slogcontext.WithValue(r.Context(), "requestId", requestId))
	}

	sloghttp.AddCustomAttributes(r, slog.String("xAmzTarget", r.Header.Get("X-Amz-Target")))

	h.proxy.ServeHTTP(w, r)
}

func isDescribeTableRequest(req *http.Request) bool {
	return strings.HasSuffix(req.Header.Get("X-Amz-Target"), ".DescribeTable")
}

func rewriteDescribeTableResponse(resp *http.Response) error {
	ctx := resp.Request.Context()
	slog.InfoContext(ctx, "attempting rewrite response JSON")

	data := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return errors.WithStack(err)
	}
	resp.Body.Close()
	resp.Body = http.NoBody

	slog.DebugContext(ctx, "raw response", slog.Any("data", data))

	it := gojqQuery.RunWithContext(ctx, data)
	out, ok := it.Next()
	if !ok {
		return errors.New("gojqQuery failed")
	}
	if err, ok := out.(error); ok {
		return errors.WithStack(err)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(out); err != nil {
		return errors.WithStack(err)
	}

	resp.Body = io.NopCloser(&buf)
	resp.ContentLength = int64(buf.Len())
	resp.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

	slog.InfoContext(ctx, "response rewrite succeeded")
	return nil
}

type loggingTransport struct {
	rt http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	if curl, err := httpcurl.IntoCurl(req); err == nil {
		slog.DebugContext(ctx, "attempting to request", slog.String("asCurl", curl))
	}

	resp, err := t.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "got a response", slog.String("status", resp.Status))
	return resp, nil
}
