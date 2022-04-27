// Copyright 2022 Ethan Marshall.
// Licensed under the ISC licence - see COPYING.
package gtemplate

import (
	"context"
	"net/http"
	"time"

	"testing"
)

const (
	TestPort         = "8080"
	TestDocumentRoot = "testing/public/"
	TestIncludesRoot = "testing/templates/"
)

type TestBroker struct{}

func (broker TestBroker) Data(path string) map[string]interface{} {
	return map[string]interface{}{
		"title":  "My Page",
		"author": "Ethan Marshall",
		"date":   time.Now(),
	}
}

func killServer(srv *http.Server) {
	time.Sleep(10 * time.Second)
	srv.Shutdown(context.Background())
}

func TestSanitizePath(t *testing.T) {
	d := [...]struct {
		Path     string
		Expected string
	}{
		{"", "/"},
		{"/a/b", "/a/b"},
		{"/a/b/", "/a/b"},
		{"/a/../", "/"},
		{"/a/../../b", "/b"},
	}

	for _, elem := range d {
		res := sanitizePath(elem.Path)
		if res != elem.Expected {
			t.Errorf("sanitizepath %q: got %q, expected %q", elem.Path, res, elem.Expected)
		}
	}
}

func TestVerifyDirectory(t *testing.T) {
	dirs := []struct {
		path  string
		valid bool
	}{
		{"testing", true},
		{"testing/", true},
		{"testing/public", true},
		{"notexist", false},
		{"testing/notexist", false},
		{"testing/public/index.gohtml", false},
	}

	for _, elem := range dirs {
		v := verifyDirectory(elem.path)
		if v != elem.valid {
			t.Errorf("verifyDirectory %q: got %v, expected %v", elem.path, v, elem.valid)
		}
	}
}

func TestServer(t *testing.T) {
	broker := TestBroker{}

	hndl, err := NewServer(TestDocumentRoot, broker)
	if err != nil {
		t.Fatalf("Server init failed: %s", err.Error())
	}

	srv := http.Server{
		Addr:    ":" + TestPort,
		Handler: hndl,
	}

	t.Log("Server starting")
	go killServer(&srv)
	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Fatalf("Server exited unexpectedly: %s", err)
	}
	t.Log("Server gracefully terminating")
}

func TestTemplateServer(t *testing.T) {
	broker := TestBroker{}

	hndl, err := NewIncludesServer(TestDocumentRoot, TestIncludesRoot, broker)
	if err != nil {
		t.Fatalf("Server start fail %s", err.Error())
	}

	srv := http.Server{
		Addr:    ":" + TestPort,
		Handler: hndl,
	}

	t.Log("Server starting")
	go killServer(&srv)
	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Fatalf("Server exited unexpectedly: %s", err)
	}
	t.Log("Server gracefully terminating")
}
