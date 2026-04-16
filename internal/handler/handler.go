package handler

import (
	"context"
	_ "embed"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/cockroachdb/errors"
	"github.com/gofrs/uuid/v5"
	"github.com/itchyny/gojq"
	sloghttp "github.com/samber/slog-http"
	"github.com/thinkgos/httpcurl"
	"github.com/utgwkk/dynamodb-local-proxy/internal/util"
)

//go:embed fill_warm_throughput.jq
var gojqQueryFillWarmThroughputStr string

var gojqQueryFillWarmThroughput = util.Must(gojq.Compile(
	util.Must(gojq.Parse(gojqQueryFillWarmThroughputStr)),
))

//go:embed rewrite_unknown_operation_exception.jq
var gojqQueryRewriteUnknownOperationExceptionStr string

var gojqQueryRewriteUnknownOperationException = util.Must(gojq.Compile(
	util.Must(gojq.Parse(gojqQueryRewriteUnknownOperationExceptionStr)),
))

type Handler struct {
	DynamoDBLocalAddr string

	httpClient *http.Client
}

func New(
	dynamodbLocalAddr string,
	httpClient *http.Client,
) *Handler {
	return &Handler{
		DynamoDBLocalAddr: dynamodbLocalAddr,

		httpClient: httpClient,
	}
}

func generateRequestId() string {
	id, err := uuid.NewV6()
	if err != nil {
		slog.Error("generateRequestId failed", slog.Any("error", err))
		return ""
	}
	return id.String()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if requestId := generateRequestId(); requestId != "" {
		r = r.WithContext(slogcontext.WithValue(r.Context(), "requestId", requestId))
	}

	sloghttp.AddCustomAttributes(r, slog.String("xAmzTarget", r.Header.Get("X-Amz-Target")))

	if err := h.serveHTTP(w, r); err != nil {
		slog.ErrorContext(r.Context(), "internal server error", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	cloneReq := h.cloneRequestForProxy(r)

	curl, err := httpcurl.IntoCurl(cloneReq)
	if err != nil {
		return errors.WithStack(err)
	}
	slog.DebugContext(
		ctx, "attempting to request",
		slog.String("asCurl", curl),
	)

	proxyResp, err := h.httpClient.Do(cloneReq)
	if err != nil {
		return errors.WithStack(err)
	}
	defer proxyResp.Body.Close()

	slog.DebugContext(
		ctx, "got an response",
		slog.String("status", proxyResp.Status),
	)

	if h.isDescribeTableRequest(cloneReq) && proxyResp.StatusCode == http.StatusOK {
		return h.rewriteDescribeTableResponse(ctx, w, proxyResp)
	} else if h.isListTagResourceRequest(cloneReq) && proxyResp.StatusCode == http.StatusBadRequest {
		return h.rewriteListTagResource400Response(ctx, w, proxyResp)
	} else {
		w.WriteHeader(proxyResp.StatusCode)
		h.copyHTTPResponseHeader(w, proxyResp)
		if _, err := io.Copy(w, proxyResp.Body); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func (h *Handler) rewriteDescribeTableResponse(ctx context.Context, w http.ResponseWriter, proxyResp *http.Response) error {
	slog.InfoContext(ctx, "attempting rewrite response JSON")
	data := map[string]any{}
	if err := json.NewDecoder(proxyResp.Body).Decode(&data); err != nil {
		return errors.WithStack(err)
	}
	slog.DebugContext(ctx, "raw response", slog.Any("data", data))

	it := gojqQueryFillWarmThroughput.RunWithContext(ctx, data)
	out, ok := it.Next()
	if !ok {
		return errors.New("gojqQuery failed")
	}
	if err, ok := out.(error); ok {
		return errors.WithStack(err)
	}

	w.WriteHeader(proxyResp.StatusCode)
	h.copyHTTPResponseHeader(w, proxyResp)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		return errors.WithStack(err)
	}
	slog.InfoContext(ctx, "response rewrite succeeded")
	return nil
}

func (h *Handler) rewriteListTagResource400Response(ctx context.Context, w http.ResponseWriter, proxyResp *http.Response) error {
	slog.InfoContext(ctx, "attempting rewrite response JSON")
	data := map[string]any{}
	if err := json.NewDecoder(proxyResp.Body).Decode(&data); err != nil {
		return errors.WithStack(err)
	}
	slog.DebugContext(ctx, "raw response", slog.Any("data", data))

	it := gojqQueryRewriteUnknownOperationException.RunWithContext(ctx, data)
	out, ok := it.Next()
	if !ok {
		return errors.New("gojqQuery failed")
	}
	if err, ok := out.(error); ok {
		return errors.WithStack(err)
	}

	w.WriteHeader(proxyResp.StatusCode)
	h.copyHTTPResponseHeader(w, proxyResp)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		return errors.WithStack(err)
	}
	slog.InfoContext(ctx, "response rewrite succeeded")
	return nil
}

func (h *Handler) cloneRequestForProxy(r *http.Request) *http.Request {
	cloneReq := r.Clone(r.Context())
	cloneReq.RequestURI = ""
	cloneReq.URL.Scheme = "http"
	cloneReq.URL.Host = h.DynamoDBLocalAddr

	cloneReq.Header.Set("User-Agent", "github.com/utgwkk/dynamodb-local-proxy")

	return cloneReq
}

func (h *Handler) copyHTTPResponseHeader(w http.ResponseWriter, proxyResp *http.Response) {
	for k, headers := range proxyResp.Header {
		if k == "Content-Length" {
			continue
		}
		for _, h := range headers {
			w.Header().Add(k, h)
		}
	}
}

func (h *Handler) isDescribeTableRequest(req *http.Request) bool {
	return strings.HasSuffix(req.Header.Get("X-Amz-Target"), ".DescribeTable")
}

func (h *Handler) isListTagResourceRequest(req *http.Request) bool {
	return strings.HasSuffix(req.Header.Get("X-Amz-Target"), ".ListTagsOfResource")
}
