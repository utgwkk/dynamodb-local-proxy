package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/cockroachdb/errors"
	"github.com/gofrs/uuid/v5"
)

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

	if err := h.serveHTTP(w, r); err != nil {
		slog.ErrorContext(r.Context(), "internal server error", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	cloneReq := h.cloneRequestForProxy(r)

	slog.DebugContext(
		ctx, "attempting to request",
		slog.String("host", cloneReq.Host),
		slog.String("method", cloneReq.Method),
		slog.String("path", cloneReq.URL.Path),
		slog.String("xAmzTarget", cloneReq.Header.Get("X-Amz-Target")),
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
	data := &struct {
		Table map[string]any
	}{}
	if err := json.NewDecoder(proxyResp.Body).Decode(data); err != nil {
		return errors.WithStack(err)
	}
	slog.DebugContext(ctx, "raw response", slog.Any("data", data))

	// append dummy WarmThroughput for table
	if _, ok := data.Table["WarmThroughput"]; !ok {
		data.Table["WarmThroughput"] = map[string]any{
			"ReadUnitsPerSecond":  5,
			"Status":              "ACTIVE",
			"WriteUnitsPerSecond": 5,
		}
	}

	// append dummy WarmThroughput for GSI
	for _, gsi := range data.Table["GlobalSecondaryIndexes"].([]any) {
		gsi2, ok := gsi.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := gsi2["WarmThroughput"]; !ok {
			gsi2["WarmThroughput"] = map[string]any{
				"ReadUnitsPerSecond":  5,
				"Status":              "ACTIVE",
				"WriteUnitsPerSecond": 5,
			}
		}
	}

	w.WriteHeader(proxyResp.StatusCode)
	h.copyHTTPResponseHeader(w, proxyResp)
	if err := json.NewEncoder(w).Encode(data); err != nil {
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
