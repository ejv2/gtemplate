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

type Broker struct{}

func (broker Broker) Data(path string) map[string]interface{} {
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

func TestServer(t *testing.T) {
	broker := Broker{}

	hndl := NewServer(DocumentRoot, broker)
	srv := http.Server{
		Addr:    ":" + Port,
		Handler: hndl,
	}

	t.Log("Server starting")
	go killServer(&srv)
	err := srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Fatalf("Server exited unexpectedly: %s", err)
	}
	t.Log("Server gracefully terminating")
}

func TestTemplateServer(t *testing.T) {
	broker := Broker{}

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

func serveConcurrent(t *testing.T, csrv chan http.Server, wait sync.WaitGroup) {
	wait.Add(1)

	broker := Broker{}

	hndl := NewServer(DocumentRoot, broker)
	srv := http.Server{
		Addr:    ":" + Port,
		Handler: hndl,
	}

	go func() { csrv <- srv }()

	t.Log("Concurrent server starting")
	err := srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Errorf("Server exited unexpectedly: %s", err)
	}

	wait.Done()
}

func TestConcurrent(t *testing.T) {
	p, count := "http://localhost:"+Port+"/test.gohtml", runtime.GOMAXPROCS(0)
	wait := sync.WaitGroup{}
	c, srv := make(chan http.Server), http.Server{}

	go serveConcurrent(t, c, wait)
	srv = <-c

	for i := 0; i < count; i++ {
		t.Logf("spawned getter %d", i)
		go fetchConcurrent(t, wait, p)
	}

	srv.Shutdown(context.Background())
	wait.Wait()
}
