package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testGateway(t *testing.T, upstream string, rate int) (*gateway, *httptest.Server) {
	t.Helper()
	var upSrv *httptest.Server
	up := upstream
	if up == "" {
		upSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"echo":"` + r.URL.Path + `"}`))
		}))
		up = upSrv.URL
	}
	g, err := newGateway(up, rate, 5)
	if err != nil {
		t.Fatal(err)
	}
	return g, upSrv
}

func TestProxyEncaminha(t *testing.T) {
	g, upSrv := testGateway(t, "", 60)
	if upSrv != nil {
		defer upSrv.Close()
	}
	srv := httptest.NewServer(g.handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), "/v1/health") {
		t.Fatalf("esperava eco de /v1/health, veio %d %s", resp.StatusCode, body)
	}
}

func TestLimiteDeVelocidade(t *testing.T) {
	g, upSrv := testGateway(t, "", 2)
	if upSrv != nil {
		defer upSrv.Close()
	}
	srv := httptest.NewServer(g.handler())
	defer srv.Close()

	derrubados := 0
	for i := 0; i < 6; i++ {
		resp, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			derrubados++
		}
		time.Sleep(100 * time.Millisecond)
	}
	if derrubados == 0 {
		t.Fatal("esperava ao menos um 429 com taxa 2/s")
	}
}

func TestUpstreamForaDoAr(t *testing.T) {
	g, _ := testGateway(t, "http://127.0.0.1:9", 60)
	srv := httptest.NewServer(g.handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("esperava 502, veio %d", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	upSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.Write([]byte(`{"db":true}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer upSrv.Close()
	g, _ := testGateway(t, upSrv.URL, 60)
	srv := httptest.NewServer(g.handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"upstream":"up"`) {
		t.Fatalf("esperava upstream up, veio %d %s", resp.StatusCode, body)
	}
}
