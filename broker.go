package gtemplate

import (
	"path"
	"strings"
	"sync"
)

// Types of registered handler
const (
	NilHandler    = iota // Default handler. Returns a nil map at all times
	ConstHandler         // Returns the same map on each invocation
	FuncHandler          // Calls a function and returns its return value
	BrokerHandler        // Passes path to separate handler and returns its return value
)

// Useful path constants
const (
	DirectoryIndex = "index.gohtml"
)

// BrokerFunc handles a request for data for a specific route
// If error is non-nil, request will return a map with only one
// entry "error" set to the error returned
type BrokerFunc func(string) (map[string]interface{}, error)

// Default data broker
// See documentation for DefaultDataBroker
var DefaultDataBroker = NewBroker()

// DefaultBroker is a scriptable request data handler.
// It matches the URL of each incoming request against a list of registered
// patterns and fetches the data (through whatever registered means) for
// this specific route. It is designed to be analogous to the http.ServeMux
// handler. See documentation for http.ServeMux for details on pattern
// matching.
type Broker struct {
	mu  sync.RWMutex                      // protects reg
	reg map[string]map[string]brokerEntry // a map of directories with path entries
}

type brokerEntry struct {
	// Type of entry (see type constants above)
	class int

	// Handler objects
	mapHandler    map[string]interface{}
	funcHandler   BrokerFunc
	brokerHandler DataBroker
}

func stringBacktrace(orig, to string) string {
	i := strings.LastIndex(orig, to)
	if i == -1 {
		return orig
	}
	return orig[:i+1]
}

func NewBroker() *Broker {
	return new(Broker)
}

func (b *Broker) Data(path string) map[string]interface{} {
	hndl, ok := b.lookupHandler(path)
	if ok {
		switch hndl.class {
		case BrokerHandler:
			return hndl.brokerHandler.Data(path)
		case ConstHandler:
			return hndl.mapHandler
		case FuncHandler:
			dat, err := hndl.funcHandler(path)
			if err != nil {
				dat = make(map[string]interface{})
				dat["error"] = err.Error()
			}

			return dat
		case NilHandler:
		default:
			panic("gtemplate: broker: unknown handler type")
		}
	}

	return nil
}

// lookupHandler traverses the handler stores and finds the most suitable entry
// If none was found, returns zero value and false, else returns entry and true
// The algorithm to lookup is as follows:
//
// For directories:
//	1) Lookup in the map
//	2) If present, return the directory's root handler
//	3) Else, return false
// For files:
//	1) Find the directory path (strip basename)
//	2) For each component of directory (starting at longest), lookup in map
//	3) If found for a component, first look for a match for whole file path
//	4) If not found for entire file path, apply for directory instead
func (b *Broker) lookupHandler(pattern string) (brokerEntry, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Is a directory
	if pattern[len(pattern)-1] == '/' {
		if e, ok := b.reg[pattern]; ok {
			if s, ok := e[pattern]; ok {
				return s, true
			}
			if s, ok := e[path.Join(pattern, DirectoryIndex)]; ok {
				return s, true
			}
		}
	} else {
		comp := pattern
		for comp != "/" {
			// We have a file, so the basename will be stripped first iteration
			comp = stringBacktrace(comp, "/")
			if e, ok := b.reg[comp]; ok {
				if s, ok := e[pattern]; ok {
					return s, true
				}

				// No match for sub-path, return dir handler
				return b.lookupHandler(comp)
			}

			comp = comp[:len(comp)-1]
		}

	}

	// No match found whatsoever
	return brokerEntry{}, false
}

func (b *Broker) registerHandler(pattern string, class int, handler interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if pattern == "" {
		panic("gtemplate: broker: empty pattern")
	}
	if handler == nil {
		panic("gtemplate: broker: nil handler")
	}

	if b.reg == nil {
		b.reg = make(map[string]map[string]brokerEntry)
		b.reg["/"] = make(map[string]brokerEntry)
	}

	entry := brokerEntry{
		class: class,
	}
	switch class {
	case BrokerHandler:
		entry.brokerHandler = handler.(DataBroker)
	case ConstHandler:
		entry.mapHandler = handler.(map[string]interface{})
	case FuncHandler:
		entry.funcHandler = handler.(BrokerFunc)
	case NilHandler:
	default:
		panic("gtemplate: broker: unknown handler type")
	}

	// Main registration code
	//
	// If entry is a directory, first check if it exists. If so, panic
	// Else, insert into hashmap with handler. Also provide handler for "index.gohtml"
	//
	// If entry is a file, find directory in hashmap
	// If not already present, create directory in hashmap
	// Then, add as handler but *do not* insert default handlers
	// If already present, simply append to existing slice
	// If already in existing slice, panic

	// Is directory
	if pattern[len(pattern)-1] == '/' {
		b.registerDirectory(pattern, entry)
	} else {
		b.registerFile(pattern, entry)
	}
}

func (b *Broker) registerDirectory(pattern string, entry brokerEntry) {
	// Path already present
	// Check for duplicates, then insert if all ok
	needIndex := true
	if m, ok := b.reg[pattern]; ok {
		if _, ok := m[pattern]; ok {
			panic("gtemplate: broker: attempted to re-register directory")
		}
		if _, ok := m[path.Join(pattern, DirectoryIndex)]; ok {
			needIndex = false
		}
	} else {
		b.reg[pattern] = make(map[string]brokerEntry)
	}

	// Add default entries
	b.reg[pattern][pattern] = entry
	if needIndex {
		b.reg[pattern][path.Join(pattern, DirectoryIndex)] = entry
	}
}

func (b *Broker) registerFile(pattern string, entry brokerEntry) {
	dir, file := path.Split(pattern)

	if file == DirectoryIndex {
		panic("gtemplate: broker: attempted to register handler for index - use directory instead")
	}

	if m, ok := b.reg[dir]; ok {
		if _, ok := m[pattern]; ok {
			panic("gtemplate: broker: attempted to re-register file")
		}
	} else {
		b.reg[dir] = make(map[string]brokerEntry)
	}

	b.reg[dir][pattern] = entry
}

// Handle registers a DataBroker to handle data requests for a route.
// Data requests are passed verbatim to this handler unchanged. What happens
// from there is not our business.
// Handle panics if broker is nil or if pattern has already been registered.
func (b *Broker) Handle(pattern string, broker DataBroker) {
	b.registerHandler(pattern, BrokerHandler, broker)
}

// HandleFunc registers a function which will be called to handle data
// requests for a route. See documentation for BrokerFunc.
// HandleFunc panics if handler is nil or if pattern has already been registered.
func (b *Broker) HandleFunc(pattern string, handler BrokerFunc) {
	b.registerHandler(pattern, FuncHandler, handler)
}

// HandleData registers a constant map which will be returned on requests for
// data for a route. The map will be accessed concurrently and must not be
// changed during execution. The best way to do this is to use a map literal.
// HandleData panics if handler is nil or if pattern has already bee registered.
func (b *Broker) HandleData(pattern string, handler map[string]interface{}) {
	b.registerHandler(pattern, ConstHandler, handler)
}
