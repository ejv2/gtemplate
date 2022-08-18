// Copyright 2022 Ethan Marshall.
// Licensed under the ISC licence - see COPYING.
//
// thp - Template Hypertext Preprocessor
// php - PHP      Hypertext Preprocessor
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/ethanv2/gtemplate"
)

var (
	root    = flag.String("root", ".", "Document root for server")
	include = flag.String("include", "", "Include root for server")
	data    = flag.String("data", "", "Data root for server")
	listen  = flag.String("listen", "", "Address on which to listen")
	cert    = flag.String("cert", "", "TLS certificate file")
	key     = flag.String("key", "", "TLS key file")
)

// ReadAll is a less portable but more specific (and dependency-avoiding)
// version of io.ReadAll.
func ReadAll(f *os.File) (buf []byte, err error) {
	b := make([]byte, 0, 512)
	for {
		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
		n, err := f.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err.Error() == "EOF" {
				err = nil
			}
			return b, err
		}
	}
}

type Broker struct {
	// Protects cache
	mut   sync.RWMutex
	cache map[string]map[string]interface{}
}

func (b *Broker) Data(path string) map[string]interface{} {
	var state, remark = "failed", "unspecified reason"
	defer func() { log.Printf("data: request for path %q %s: %s", path, state, remark) }()

	dfile := path + ".data"
	p := filepath.Join(*data, dfile)

	// Check for cache hit - return early
	b.mut.RLock()
	if val, ok := b.cache[p]; ok {
		defer b.mut.RUnlock()

		state, remark = "success", "cache hit"
		return val
	}
	b.mut.RUnlock()

	f, err := os.Open(p)
	if err != nil {
		state, remark = "failed", "no associated data"
		return nil
	}

	buf, err := ReadAll(f)
	if err != nil {
		state, remark = "failed", "error reading data"
		return nil
	}

	res := make(map[string]interface{})
	err = json.Unmarshal(buf, &res)
	if err != nil {
		state, remark = "failed", "malformed data file: "+err.Error()
		return nil
	}

	b.mut.Lock()
	if b.cache == nil {
		b.cache = make(map[string]map[string]interface{})
	}

	b.cache[p] = res
	b.mut.Unlock()

	// Yay!
	state, remark = "success", "loaded datafile"
	return res
}

func main() {
	flag.Parse()
	if (*cert == "" && *key != "") || (*cert != "" && *key == "") {
		log.Fatalln("tls: must provide both certificate and key")
	}
	if *data == "" {
		*data = *root
	}

	log.Println("template engine starting")
	var err error
	var hndl http.Handler
	broker := new(Broker)
	if *include != "" {
		hndl, err = gtemplate.NewIncludesServer(*root, *include, broker)
	} else {
		hndl, err = gtemplate.NewServer(*root, broker)
	}

	if err != nil {
		log.Fatalf("template engine error: %s", err.Error())
	}

	log.Println("server starting")
	if *cert != "" {
		if *listen == "" {
			*listen = ":443"
		}

		err = http.ListenAndServeTLS(*listen, *cert, *key, hndl)
	} else {
		if *listen == "" {
			*listen = ":80"
		}

		err = http.ListenAndServe(*listen, hndl)
	}

	if err != http.ErrServerClosed {
		log.Fatalf("fatal server error: %s", err.Error())
	}
	log.Println("server terminating gracefully")
}
