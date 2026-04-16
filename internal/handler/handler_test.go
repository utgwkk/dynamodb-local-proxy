package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	slogcontext "github.com/PumpkinSeed/slog-context"
	"github.com/utgwkk/dynamodb-local-proxy/internal/httptestmock"
)

func newTestHandler(t *testing.T, filename string) *Handler {
	srv := httptestmock.NewServer(t, filename)
	addr := strings.TrimPrefix(srv.URL, "http://")
	h := New(addr, srv.Client())
	return h
}

type WarmThroughput struct {
	ReadUnitsPerSecond  int
	Status              string
	WriteUnitsPerSecond int
}

type GlobalSecondaryIndex struct {
	IndexName      string          `json:"IndexName"`
	WarmThroughput *WarmThroughput `json:"WarmThroughput"`
}

type Table struct {
	GlobalSecondaryIndexes []*GlobalSecondaryIndex `json:"GlobalSecondaryIndexes"`

	WarmThroughput *WarmThroughput `json:"WarmThroughput"`
}

type DescribeTableResponse struct {
	Table *Table
}

func checkRestFieldRemains(t *testing.T, body []byte) {
	t.Helper()

	// check rest field remains
	var decoded struct {
		Table map[string]any
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal("failed to parse response as JSON:", err.Error())
	}
	for _, k := range []string{
		"AttributeDefinitions",
		"TableName",
		"KeySchema",
		"TableStatus",
		"CreationDateTime",
		"ProvisionedThroughput",
		"TableSizeBytes",
		"ItemCount",
		"TableArn",
		"DeletionProtectionEnabled",
	} {
		if _, ok := decoded.Table[k]; !ok {
			t.Errorf("%s field not present", k)
		}
	}
}

func TestHandler(t *testing.T) {
	t.Parallel()
	slog.SetDefault(
		slog.New(
			slogcontext.NewHandler(
				slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}),
			),
		),
	)

	t.Run("TableExists", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t, "TableExists.txt")
		rec := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost:8001",
			strings.NewReader(`{"TableName":"test"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeTable")

		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Logf("unexpected status code: %d (want %d)", rec.Code, http.StatusOK)
			t.Log("body:", rec.Body.String())
			t.FailNow()
		}

		var decoded DescribeTableResponse
		bodyCopy := rec.Body.Bytes()
		if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
			t.Fatal("failed to parse response as JSON:", err.Error())
		}

		// check table WarmThroughput
		if decoded.Table.WarmThroughput == nil {
			t.Error("table does not have WarmThroughput Field")
		}

		// check GSI WarmThroughput
		if len(decoded.Table.GlobalSecondaryIndexes) == 0 {
			t.Fatal("Table.GlobalSecondaryIndexes must not be empty")
		}
		for _, gsi := range decoded.Table.GlobalSecondaryIndexes {
			if gsi.WarmThroughput == nil {
				t.Errorf("GSI %s does not have WarmThroughput Field", gsi.IndexName)
			}
		}

		checkRestFieldRemains(t, bodyCopy)
	})

	t.Run("TableCreatedButNoIndex", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t, "TableCreatedButNoIndex.txt")
		rec := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost:8001",
			strings.NewReader(`{"TableName":"test"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeTable")

		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Logf("unexpected status code: %d (want %d)", rec.Code, http.StatusOK)
			t.Log("body:", rec.Body.String())
			t.FailNow()
		}

		var decoded DescribeTableResponse
		bodyCopy := rec.Body.Bytes()
		if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
			t.Fatal("failed to parse response as JSON:", err.Error())
		}

		// check table WarmThroughput
		if decoded.Table.WarmThroughput == nil {
			t.Error("table does not have WarmThroughput Field")
		}

		checkRestFieldRemains(t, bodyCopy)
	})

	t.Run("AlreadyFilled", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t, "AlreadyFilled.txt")
		rec := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"http://localhost:8001",
			strings.NewReader(`{"TableName":"test"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeTable")

		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Logf("unexpected status code: %d (want %d)", rec.Code, http.StatusOK)
			t.Log("body:", rec.Body.String())
			t.FailNow()
		}

		var decoded DescribeTableResponse
		bodyCopy := rec.Body.Bytes()
		if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
			t.Fatal("failed to parse response as JSON:", err.Error())
		}

		// check table WarmThroughput
		want := &WarmThroughput{
			ReadUnitsPerSecond:  2,
			Status:              "ACTIVE",
			WriteUnitsPerSecond: 2,
		}
		if !reflect.DeepEqual(decoded.Table.WarmThroughput, want) {
			t.Errorf("WarmThroughput mismatch\nwant: %+v", *want)
		}

		// check GSI WarmThroughput
		if len(decoded.Table.GlobalSecondaryIndexes) == 0 {
			t.Fatal("Table.GlobalSecondaryIndexes must not be empty")
		}
		for _, gsi := range decoded.Table.GlobalSecondaryIndexes {
			if !reflect.DeepEqual(gsi.WarmThroughput, want) {
				t.Errorf("WarmThroughput mismatch\nwant: %+v", *want)
			}
		}

		checkRestFieldRemains(t, bodyCopy)
	})

	t.Run("TableNotFound", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t, "TableNotFound.txt")
		rec := httptest.NewRecorder()
		req, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://localhost:8001",
			strings.NewReader(`{"TableName":"test"}`),
		)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Amz-Target", "DynamoDB_20120810.DescribeTable")

		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("unexpected status code: %d (want %d)", rec.Code, http.StatusBadRequest)
			t.Error("Body:", rec.Body.String())
		}
	})
}
