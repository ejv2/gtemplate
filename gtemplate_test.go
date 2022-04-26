// Copyright 2022 Ethan Marshall.
// Licensed under the ISC licence - see COPYING.
package gtemplate

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"testing"
)

const (
	Port         = "8080"
	DocumentRoot = "testing/public/"
	IncludesRoot = "testing/templates/"
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

	hndl, err := NewServer(DocumentRoot, broker)
	if err != nil {
		t.Fatalf("Server init failed: %s", err.Error())
	}

	srv := http.Server{
		Addr:    ":" + Port,
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

	hndl, err := NewIncludesServer(DocumentRoot, IncludesRoot, broker)
	if err != nil {
		t.Fatalf("Server start fail %s", err.Error())
	}

	srv := http.Server{
		Addr:    ":" + Port,
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

func TestDefaultBroker(t *testing.T) {
	broker := TestBroker{}
	Handle("/", broker)
	HandleData("/sub/", map[string]interface{}{
		"title":  "Data bound through the sub directory",
		"author": "github.com/ethanv2/gtemplate",
		"date":   time.Time{},
	})
	HandleFunc("/temp.gohtml", func(path string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"title":  "If you can see this, it works!",
			"author": "ethan_v2",
			"date":   time.Now().Add(24 * time.Hour),
		}, nil
	})

	hndl, err := NewIncludesServer(DocumentRoot, IncludesRoot, nil)
	if err != nil {
		t.Errorf("Server init failed: %s", err.Error())
		return
	}

	srv := http.Server{
		Addr:    ":" + Port,
		Handler: hndl,
	}

	t.Log("DefaultDataBroker server starting")
	go killServer(&srv)
	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Errorf("Server exited unexpectedly: %s", err)
	}
}

func fetchConcurrent(t *testing.T, wait sync.WaitGroup, url string) {
	wait.Add(1)

	b, err := http.Get(url)
	if err != nil {
		t.Errorf("unexpected request error: %s", err.Error())
		return
	} else if b.StatusCode != 200 {
		t.Errorf("unexpected response code %d", b.StatusCode)
		return
	}

	buf := make([]byte, 18)
	_, err = b.Body.Read(buf)
	if err != nil {
		t.Errorf("unexpected request error: %s", err.Error())
	} else if string(buf) != "should be returned" {
		t.Errorf("incorrect body returned: %s", string(buf))
	}

	wait.Done()
}

func serveConcurrent(t *testing.T, wait sync.WaitGroup) {
	broker := TestBroker{}

	hndl, err := NewServer(DocumentRoot, broker)
	if err != nil {
		t.Errorf("Server init failed: %s", err.Error())
		return
	}

	srv := http.Server{
		Addr:    ":" + Port,
		Handler: hndl,
	}

	t.Log("Concurrent server starting")
	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Errorf("Server exited unexpectedly: %s", err)
	}
}

func TestConcurrent(t *testing.T) {
	p, count := "http://localhost:"+Port+"/test.gohtml", runtime.GOMAXPROCS(0)
	wait := sync.WaitGroup{}

	go serveConcurrent(t, wait)

	for i := 0; i < count; i++ {
		t.Logf("spawned getter %d", i)
		go fetchConcurrent(t, wait, p)
	}

	wait.Wait()
}
