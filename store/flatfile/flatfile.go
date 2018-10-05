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

// Package flatfile is a flat file TiddlerStore backend.
package flatfile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gitlab.com/opennota/widdly/store"
)

var sep = string(filepath.Separator)

// flatFileStore is a flat file store for tiddlers.
type flatFileStore struct {
	storePath          string
	tiddlersPath       string
	tiddlerHistoryPath string
	m                  sync.RWMutex
}

func init() {
	if store.MustOpen != nil {
		panic("attempt to use two different backends at the same time!")
	}
	store.MustOpen = MustOpen
}

// MustOpen opens a flat file store at storePath, creating directories if needed,
// and returns a TiddlerStore.
// MustOpen panics if there is an error.
func MustOpen(storePath string) store.TiddlerStore {
	if err := os.MkdirAll(storePath, 0755); err != nil {
		panic(err)
	}

	tiddlersPath := filepath.Join(storePath, "tiddlers")
	if err := os.MkdirAll(tiddlersPath, 0755); err != nil {
		panic(err)
	}

	tiddlerHistoryPath := filepath.Join(storePath, "tiddlerHistory")
	if err := os.MkdirAll(tiddlerHistoryPath, 0755); err != nil {
		panic(err)
	}

	return &flatFileStore{
		storePath:          storePath,
		tiddlersPath:       tiddlersPath,
		tiddlerHistoryPath: tiddlerHistoryPath,
	}
}

var keySanitizer = strings.NewReplacer(
	"/", "_",
	`\`, "_",
	":", "_",
	"*", "_",
	"?", "_",
	`"`, "_",
	">", "_",
	"<", "_",
	"|", "_",
	"[", "_",
	"]", "_",
)

func sanitizeKey(key string) string { return keySanitizer.Replace(key) }

// Get retrieves a tiddler from the store by key (title).
func (s *flatFileStore) Get(_ context.Context, key string) (store.Tiddler, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	skey := sanitizeKey(key)
	metaPath := filepath.Join(s.tiddlersPath, skey+".meta")
	meta, err := ioutil.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return store.Tiddler{}, store.ErrNotFound
		}
		return store.Tiddler{}, err
	}

	tidPath := filepath.Join(s.tiddlersPath, skey+".tid")
	tiddler, err := ioutil.ReadFile(tidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return store.Tiddler{}, store.ErrNotFound
		}
		return store.Tiddler{}, err
	}

	return store.Tiddler{
		Key:      key,
		Meta:     meta,
		Text:     string(tiddler),
		WithText: true,
	}, nil
}

// All retrieves all the tiddlers (mostly skinny) from the store.
// Special tiddlers (like global macros) are returned fat.
func (s *flatFileStore) All(_ context.Context) ([]store.Tiddler, error) {
	s.m.RLock()
	defer s.m.RUnlock()

	files, err := filepath.Glob(filepath.Join(s.tiddlersPath, "*.meta"))
	if err != nil {
		return nil, err
	}

	tiddlers := []store.Tiddler{}
	for _, file := range files {
		meta, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}
		var t store.Tiddler
		if err := json.Unmarshal(meta, &struct {
			Title *string
		}{&t.Key}); err != nil {
			continue
		}
		t.Meta = meta
		if bytes.Contains(meta, []byte(`"$:/tags/Macro"`)) {
			tiddlerPath := strings.TrimSuffix(file, filepath.Ext(file))
			tiddler, err := ioutil.ReadFile(tiddlerPath + ".tid")
			if err != nil {
				continue
			}
			t.Text = string(tiddler)
			t.WithText = true
		}
		tiddlers = append(tiddlers, t)
	}
	return tiddlers, nil
}

func (s *flatFileStore) nextRevision(key string) int {
	files, _ := filepath.Glob(filepath.Join(s.tiddlerHistoryPath, "*"+key+"#[1-9]*"))
	r := regexp.MustCompile(regexp.QuoteMeta(sep+key) + `#(\d+)$`)
	maxRev := 0
	for _, file := range files {
		m := r.FindStringSubmatch(file)
		if m == nil {
			continue
		}
		rev, _ := strconv.Atoi(m[1])
		if rev > maxRev {
			maxRev = rev
		}
	}
	return maxRev + 1
}

func skipHistory(key string) bool {
	return key == "$:/StoryList" || strings.HasPrefix(key, "Draft of ")
}

// Put saves tiddler to the store, incrementing and returning revision.
// The tiddler is also written to the tiddler_history bucket.
func (s *flatFileStore) Put(_ context.Context, tiddler store.Tiddler) (int, error) {
	s.m.Lock()
	defer s.m.Unlock()

	skey := sanitizeKey(tiddler.Key)

	var js map[string]interface{}
	err := json.Unmarshal(tiddler.Meta, &js)
	if err != nil {
		return 0, err
	}

	tidPath := filepath.Join(s.tiddlersPath, skey+".tid")
	if err := ioutil.WriteFile(tidPath, []byte(tiddler.Text), 0644); err != nil {
		return 0, err
	}

	rev := s.nextRevision(skey)

	metaPath := filepath.Join(s.tiddlersPath, skey+".meta")
	js["revision"] = rev
	data, _ := json.Marshal(js)
	if err := ioutil.WriteFile(metaPath, tiddler.Meta, 0644); err != nil {
		return 0, err
	}

	if !skipHistory(tiddler.Key) {
		histPath := filepath.Join(s.tiddlerHistoryPath, fmt.Sprintf("%s#%d", skey, rev))
		js["text"] = tiddler.Text
		data, _ = json.Marshal(js)
		if err := ioutil.WriteFile(histPath, data, 0644); err != nil {
			return 0, err
		}
	}

	return rev, nil
}

// Delete deletes a tiddler with the given key (title) from the store.
func (s *flatFileStore) Delete(ctx context.Context, key string) error {
	s.m.Lock()
	defer s.m.Unlock()

	skey := sanitizeKey(key)
	if !skipHistory(key) {
		rev := s.nextRevision(skey)
		histPath := filepath.Join(s.tiddlerHistoryPath, fmt.Sprintf("%s#%d", skey, rev))
		if err := ioutil.WriteFile(histPath, nil, 0644); err != nil {
			return err
		}
	}
	if err := os.Remove(filepath.Join(s.tiddlersPath, skey+".meta")); err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.tiddlersPath, skey+".tid"))
}
