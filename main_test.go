package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func execute(args []string, serverURL string) (int, string, string) {
	env := map[string]string{
		"ARCHEPELAG_SERVER_URL": serverURL,
		"ARCHEPELAG_TOKEN":      "test-token",
	}
	var stdout, stderr bytes.Buffer
	code := run(args, func(key string) string { return env[key] }, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestListProductsFollowsPages(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/products" ||
			r.Header.Get("Authorization") != "Bearer test-token" ||
			r.URL.Query().Get("search") != "Платёж & Co" || r.URL.Query().Get("pageSize") != "100" ||
			r.URL.Query().Get("page") != fmt.Sprint(page) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		fmt.Fprintf(w, `{"products":[{"id":"%d","name":"Продукт","extra":9007199254740993}],"pagination":{"page":%d,"totalPages":2}}`,
			page, page)
	}))
	defer server.Close()

	code, stdout, stderr := execute([]string{"products", "list", "--search", "Платёж & Co", "--json"}, server.URL)
	var products []json.RawMessage
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &products) != nil || len(products) != 2 ||
		requests.Load() != 2 || !strings.Contains(stdout, "9007199254740993") {
		t.Fatalf("code=%d requests=%d stdout=%q stderr=%q", code, requests.Load(), stdout, stderr)
	}
}

func TestGetProductFromInstallationPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/instance/api/v1/products/платёж" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"product":{"id":"uuid","name":"Платёж","key":"PAY","description":"Приём платежей"}}`)
	}))
	defer server.Close()

	code, stdout, stderr := execute([]string{"products", "get", "платёж"}, server.URL+"/instance/")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Платёж") || !strings.Contains(stdout, "Приём платежей") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestListErrorLeavesStdoutEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			http.Error(w, "test-token", http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"products":[{"id":"1","name":"First"}],"pagination":{"page":1,"totalPages":2}}`)
	}))
	defer server.Close()

	code, stdout, stderr := execute([]string{"products", "list", "--json"}, server.URL)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "401") || strings.Contains(stderr, "test-token") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestGetRefusesRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products/uuid" {
			t.Errorf("followed redirect to %s", r.URL.Path)
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer server.Close()

	code, stdout, stderr := execute([]string{"products", "get", "uuid"}, server.URL)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "302") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
