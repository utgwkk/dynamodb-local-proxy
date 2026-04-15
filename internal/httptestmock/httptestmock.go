package httptestmock

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
)

func NewServer(t testing.TB, filename string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open(path.Join("testdata", filename))
		if err != nil {
			t.Fatalf("open %s failed:", err)
		}
		defer f.Close()

		resp, err := http.ReadResponse(bufio.NewReader(f), nil)
		if err != nil {
			t.Fatal("construct response failed:", err)
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}

		if _, err := io.Copy(w, resp.Body); err != nil {
			t.Log("failed to write response:", err.Error())
		}
	}))

	t.Cleanup(srv.Close)
	return srv
}
