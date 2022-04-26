// Copyright 2022 Ethan Marshall.
// Licensed under the ISC licence - see COPYING.
package gtemplate

import (
	"errors"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
)

// TemplateServer returned errors
var (
	ErrRootInvalid     = errors.New("gtemplate: root: invalid root directory")
	ErrIncludesInvalid = errors.New("gtemplate: includes: invalid includes directory")
	ErrAlreadyParsed   = errors.New("gtemplate: attempted to re-parse for path")
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

// verifyDirectory checks if a path exists and is a directory
func verifyDirectory(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}

	if !info.IsDir() {
		return false
	}

	return true
}

// loadIncludes traverses and loads any potential include templates
// from the includeRoot at path
func (srv *TemplateServer) loadIncludes(path string) error {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) || errors.Is(err, os.ErrInvalid) {
		return ErrIncludesInvalid
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

// loadTemplate loads and caches (thread safely) a template file located
// at path
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

// ServeHTTP loads, parses (if not already cached) and serves a template
// specified in the requests URL. Can be safely called in parallel, as is
// done by http.Server
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
			http.Error(w, "404 not found", http.StatusNotFound)
			return
		}
	}

	data := srv.broker.Data(p)
	err := srv.templates[p].ExecuteTemplate(w, path.Base(p), data)

	if err != nil {
		http.Error(w, "500 internal error\n\t"+err.Error(), http.StatusInternalServerError)
	}
}

// NewServer instantiates a new TemplateServer instance which can be
// used with http.Server as a handler
func NewServer(root string, data DataBroker) (http.Handler, error) {
	if !verifyDirectory(root) {
		return nil, ErrRootInvalid
	}
	if data == nil {
		data = DefaultDataBroker
	}

	srv := &TemplateServer{
		broker:    data,
		templates: make(map[string]*template.Template),
		root:      root,
	}

	return srv, nil
}

// NewIncludesServer instantiates a new TemplateServer instance with
// includes support, meaning that templates in includeRoot can be used
// by any other executing template. Templates in the root still cannot
// execute each other. The instance can be used with http.Server as a
// handler. Error is returned if root or includeRoot are invalid directories
func NewIncludesServer(root string, includeRoot string, data DataBroker) (http.Handler, error) {
	if data == nil {
		data = DefaultDataBroker
	}

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
