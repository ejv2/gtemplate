package gtemplate

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultBroker(t *testing.T) {
	broker := TestBroker{}
	Handle("/", broker)
	HandleData("/sub/", map[string]interface{}{
		"title":  "Data bound through the sub directory",
		"author": "github.com/ejv2/gtemplate",
		"date":   time.Time{},
	})
	HandleFunc("/temp.gohtml", func(path string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"title":  "If you can see this, it works!",
			"author": "ethan_v2",
			"date":   time.Now().Add(24 * time.Hour),
		}, nil
	})

	hndl, err := NewIncludesServer(TestDocumentRoot, TestIncludesRoot, nil)
	if err != nil {
		t.Errorf("Server init failed: %s", err.Error())
		return
	}

	srv := http.Server{
		Addr:    ":" + TestPort,
		Handler: hndl,
	}

	t.Log("DefaultDataBroker server starting")
	go killServer(&srv)
	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		t.Errorf("Server exited unexpectedly: %s", err)
	}
}

func BenchmarkHandlerLookup(b *testing.B) {
	hndl := Broker{}
	paths := []string{
		"/",
		"/a.gohtml",
		"/temp.gohtml",
		"/temp/",
		"/temp/a.gohtml",
		"/temp/temp.gohtml",
	}
	hfunc := func(path string) (map[string]interface{}, error) {
		panic("not reached")
	}

	for _, elem := range paths {
		hndl.HandleFunc(elem, hfunc)
	}

	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]

		hndl.lookupHandler(path)
	}
}
