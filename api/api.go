// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General
// Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package api registers needed HTTP handlers.
package api

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/opennota/widdly/store"
)

// TiddlyWikiStatus contains all instance relevant information
type TiddlyWikiStatus struct {
	Username string           `json:"username"`
	ReadOnly bool             `json:"read_only"`
	Space    TiddlyWikiRecipe `json:"space"`
}

// TiddlyWikiRecipe represents a bag
type TiddlyWikiRecipe struct {
	Recipe string `json:"recipe"`
}

var (
	// Store should point to an implementation of TiddlerStore.
	Store store.TiddlerStore

	// Authenticate is a hook that lets the client of the package to
	// provide some authentication.
	// Authenticate should write to the ResponseWriter iff the user
	// may not access the endpoint.
	Authenticate func(http.ResponseWriter, *http.Request)

	// ServeIndex is a callback that should serve the index page.
	ServeIndex = func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serving index")
		http.ServeFile(w, r, "index.html")
	}

	// ServeMux is a HTTP router (multiplexor)
	ServeMux = http.NewServeMux()

	// ReadOnly defines if TW instance should run in read-only mode
	ReadOnly = false

	// InstanceStatus contains all instance relevant information
	InstanceStatus TiddlyWikiStatus

	// DefaultURLPath specifies default URL path
	DefaultURLPath = ""
)

// InitRoutes inits the mux routes
func InitRoutes() {
	log.Println("Default URL Path: ", DefaultURLPath)
	ServeMux.HandleFunc(fmt.Sprintf("%s/", DefaultURLPath), withLoggingAndAuth(index))
	ServeMux.HandleFunc(fmt.Sprintf("%s/status", DefaultURLPath), withLoggingAndAuth(status))
	ServeMux.HandleFunc(fmt.Sprintf("%s/recipes/all/tiddlers.json", DefaultURLPath), withLoggingAndAuth(list))
	ServeMux.HandleFunc(fmt.Sprintf("%s/recipes/all/tiddlers/", DefaultURLPath), withLoggingAndAuth(tiddler))
	ServeMux.HandleFunc(fmt.Sprintf("%s/bags/bag/tiddlers/", DefaultURLPath), withLoggingAndAuth(remove))
}

// SetStatus sets status response
func SetStatus() {
	InstanceStatus = TiddlyWikiStatus{
		Username: "me",
		ReadOnly: ReadOnly,
		Space: TiddlyWikiRecipe{
			Recipe: "all",
		},
	}
}

// internalError logs err to the standard error and returns HTTP 500 Internal Server Error.
func internalError(w http.ResponseWriter, err error) {
	log.Println("ERR", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

// logRequest logs the incoming request.
func logRequest(r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	log.Println(host, r.Method, r.URL, r.Referer(), r.UserAgent())
}

// withLogging is a logging middleware.
func withLogging(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		f(w, r)
	}
}

type responseWriter struct {
	http.ResponseWriter
	written bool
}

func (w *responseWriter) Write(p []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(p)
}

func (w *responseWriter) WriteHeader(status int) {
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}

// withAuth is an authentication middleware.
func withAuth(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if Authenticate == nil {
			f(w, r)
		} else {
			rw := responseWriter{
				ResponseWriter: w,
			}
			Authenticate(&rw, r)
			if !rw.written {
				f(w, r)
			}
		}
	}
}

func withLoggingAndAuth(f http.HandlerFunc) http.HandlerFunc {
	return withAuth(withLogging(f))
}

// index serves the index page.
func index(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Println("URL Path: ", r.URL.Path)
	if DefaultURLPath != "" {
		if r.URL.Path != DefaultURLPath {
			http.NotFound(w, r)
			return
		}
	} else {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
	}
	ServeIndex(w, r)
}

// status serves the status JSON.
func status(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Send status to client
	w.Header().Set("Content-Type", "application/json")
	jsonData, err := json.Marshal(InstanceStatus)
	if err != nil {
		http.Error(w, "couldn't marshal status information", http.StatusInternalServerError)
	}
	w.Write(jsonData)
}

// list serves a JSON list of (mostly) skinny tiddlers.
func list(w http.ResponseWriter, r *http.Request) {
	tiddlers, err := Store.All(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(tiddlers)
	if err != nil {
		log.Println("ERR", err)
	}
}

// getTiddler serves a fat tiddler.
func getTiddler(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/recipes/all/tiddlers/", DefaultURLPath))

	t, err := Store.Get(r.Context(), key)
	if err != nil {
		if err == store.ErrNotFound {
			http.NotFound(w, r)
		} else {
			internalError(w, err)
		}
		return
	}

	data, err := t.MarshalJSON()
	if err != nil {
		internalError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// putTiddler saves a tiddler.
func putTiddler(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/recipes/all/tiddlers/", DefaultURLPath))

	var js map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&js)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	io.Copy(ioutil.Discard, r.Body)

	js["bag"] = "bag"

	text, _ := js["text"].(string)
	delete(js, "text")

	meta, err := json.Marshal(js)
	if err != nil {
		internalError(w, err)
		return
	}

	rev, err := Store.Put(r.Context(), store.Tiddler{
		Key:  key,
		Meta: meta,
		Text: text,
	})
	if err != nil {
		internalError(w, err)
		return
	}

	etag := fmt.Sprintf(`"bag/%s/%d:%032x"`, url.QueryEscape(key), rev, md5.Sum(meta))
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusNoContent)
}

func tiddler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getTiddler(w, r)
	case "PUT":
		putTiddler(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// remove removes a tiddler.
func remove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/bags/bag/tiddlers/", DefaultURLPath))
	err := Store.Delete(r.Context(), key)
	if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
