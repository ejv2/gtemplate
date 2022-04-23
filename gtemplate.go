// Copyright 2022 Ethan Marshall
// Licensed under the ISC licence - see COPYING
package gtemplate

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
)

// TemplateServer returned errors
var (
	ErrIncludesNotExist = errors.New("gtemplate: includes: no such directory")
	ErrIncludesNotDir   = errors.New("gtemplate: includes: not a directory")
	ErrAlreadyParsed    = errors.New("gtemplate: attempted to re-parse for path")
)

// A DataBroker is responsible for mapping data to bind to a
// specific path, passed as an argument to Data.
// This allows different or the same data to be provided based
// on server state or the page being accessed.
type DataBroker interface {
	Data(path string) map[string]interface{}
}

// A TemplateServer is analogous to a Go standard file server, but
// which passes files through the template engine first, intended
// for simple dynamic sites. It acts as the http.Handler for a
// net/http http.Server instance, which allows for files to be
// routed using templating logic. Templates are loaded from disk upon
// first request and the compilation result cached in a map of paths.
type TemplateServer struct {
	broker    DataBroker
	mut       sync.RWMutex
	templates map[string]*template.Template
	includes  []string
	root      string
}

func sanitizePath(p string) string {
	if p == "" {
		return "/"
	}

	if p[0] != '/' {
		p = "/" + p
	}

	return path.Clean(p)
}

func (srv *TemplateServer) loadIncludes(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return ErrIncludesNotExist
	} else if errors.Is(err, os.ErrInvalid) {
		return ErrIncludesNotDir
	}

	for _, elem := range entries {
		if elem.Type().IsDir() {
			err = srv.loadIncludes(filepath.Join(path, elem.Name()))
			if err != nil {
				return err
			}

			continue
		}

		srv.includes = append(srv.includes, filepath.Join(path, elem.Name()))
	}

	return nil
}

func (srv *TemplateServer) loadTemplate(path string) error {
	files := make([]string, 0, len(srv.includes)+1)
	files = append(files, srv.includes...)
	files = append(files, filepath.Join(srv.root, path))

	srv.mut.Lock()
	defer srv.mut.Unlock()

	_, ok := srv.templates[path]
	if ok {
		return ErrAlreadyParsed
	}

	var err error
	srv.templates[path], err = template.New(path).ParseFiles(files...)
	if err != nil {
		delete(srv.templates, path)
		return err
	}

	return nil
}

func (srv *TemplateServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		r.URL.Path = "/index.gohtml"
	}
	p := sanitizePath(r.URL.Path)

	srv.mut.RLock()
	defer srv.mut.RUnlock()
	if _, ok := srv.templates[p]; !ok {
		srv.mut.RUnlock()
		err := srv.loadTemplate(p)
		srv.mut.RLock()

		if err != nil {
			w.WriteHeader(404)
			fmt.Fprintln(w, "404 not found")
			return
		}
	}

	data := srv.broker.Data(p)
	err := srv.templates[p].ExecuteTemplate(w, p[1:], data)

	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "500 internal error\n\t%s", err.Error())
	}

}

// NewServer instantiates a new TemplateServer instance which can be
// used with http.Server as a handler
func NewServer(root string, data DataBroker) http.Handler {
	srv := &TemplateServer{
		broker:    data,
		templates: make(map[string]*template.Template),
		root:      root,
	}

	return srv
}

func NewIncludesServer(root string, includeRoot string, data DataBroker) (http.Handler, error) {
	srv := &TemplateServer{
		broker:    data,
		templates: make(map[string]*template.Template),
		root:      root,
	}

	err := srv.loadIncludes(includeRoot)
	if err != nil {
		return nil, err
	}

	return srv, nil
}