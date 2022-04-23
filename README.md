# gtemplate - simple Go html/templates server

``gtemplate`` is a dead simple implementation of a template-powered dynamic HTTP server.

The main page view is provided by on-disk HTML templates. Data is provided by a "data broker", that takes in a path and returns a map of string to empty interface and can be used to programmatically provide dynamic data for the loaded template.

## Features

1. Implements ``http.Handler``, so is easily used with the standard http.Server and http.ServeMux
1. Can be used with ``http.Handle`` to filter what requests are templated and to serve specific directories
1. Supports custom, dynamic data binding to build interactive, server-sided webpages
1. Very minimal and performant, with template caching meaning that the same page will never be parsed twice, but many different pieces of data can be used
1. Template locations are specified by directory and can be any filesystem path. Requests outside this path will not be served - directory traversal is *impossible* (I hope!)

## Example

```go
package main

import (
	"log"
	"net/http"

	"github.com/ethanv2/gtemplate"
)

const (
	// Location of gohtml template files
	// Does not need to be the same as the URL path handled
	DocumentRoot = "somedir"
	// Port for the http.Server to listen on
	Port         = "8080"
)

// Broker is literally the most simple data broker possible
// and only returns a constant map.
// Regardless, it needs to be a type. In a real example, internal
// or cached data is likely here. This can be anything you want.
type Broker struct{}

// Data returned from data will be set to the '.' in the
// template execution.
// Keys in map will be available under '.' as though fields
// in a struct. In this case, {{.Name}} or {{range .Relatives}}
func (b Broker) Data(path string) map[string]interface{} {
	return map[string]interface{}{
		"Name": "ethanv2",
		"DOB": 2006,
		"Relatives": [...]string{
			"Dave",
			"Bob",
			"Phil",
		},
	}

}

func main() {
	broker := Broker{}

	// Returns a new plain handler, without any include
	// templates
	hndl, err := gtemplate.NewServer(DocumentRoot, broker)
	if err != nil {
		panic(err)
	}

	srv := http.Server{
		Addr:    ":" + Port,
		Handler: hndl,
	}

	// The rest of the server is a standard net/http.Server
	// instance
	err = srv.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatalf("Server exited unexpectedly: %s", err.Error())
	}
}
```

This server will start and serve the template files in the directory specified in ``DocumentRoot`` mapped to the root of the server. The templates will be parsed and bound to any data as selected and returned by the custom broker passed in to ``NewServer``. This is the minimal example of a working server.

## Limitations and Pitfalls

1. If multiple pages are hit at once which require the parsing of a template, only one of those requests will be served at a time (requests will be serialised). Other cached requests will continue as normal. This is to prevent issues in parsing of templates and in caching.
1. Every page in the ``DocumentRoot`` will be treated as a gohtml document and will be parsed accordingly. There is no support for filetype detection. Structure your directories carefully!
1. Data broker is frequently called concurrently. In fact, in ideal scenarios, data broker will be serving several requests at once with no need to re-parse the template. Be sure to use locking or channels where appropriate to manage this!

## Why?

A question I asked myself.

I just wanted something really simple that follows the style of http.FileServer without the need to import some massive library like mux. These's nothing wrong with using this at all, especially if you're already using it. You can even use these two in combination! But, I just needed something small and simple, so this is what I ended up with.
