package scan

import (
	"io/fs"
	"sort"
	"sync"

	"github.com/whoisnjoguu/elostirion/pkg/model"
)

// Scanner extracts a namespace of facts from a repo filesystem.
type Scanner interface {
	Name() string // fact namespace this scanner writes
	Scan(fsys fs.FS, facts *model.Facts) error
}

// Meta is an optional interface a scanner implements to associate itself with a
// language and the files that identify a repository of that language
type Meta interface {
	Language() string  // empty string means language-agnostic
	Markers() []string // filenames whose presence in a repo root identifies it as a repo of this language
}

// languageOf returns a scanner's language
func languageOf(s Scanner) string {
	if m, ok := s.(Meta); ok {
		return m.Language()
	}
	return ""
}

var (
	mu       sync.RWMutex
	registry = map[string]Scanner{}
)

// Register makes a scanner available by name
func Register(s Scanner) {
	mu.Lock()
	defer mu.Unlock()
	registry[s.Name()] = s
}

// Registered returns the registered scanners sorted by name.
func Registered() []Scanner {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Scanner, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Get returns a registered scanner by name.
func Get(name string) (Scanner, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := registry[name]
	return s, ok
}

// Languages returns the distinct languages of registered scanners
func Languages() []string {
	seen := map[string]bool{}
	for _, s := range Registered() {
		if lang := languageOf(s); lang != "" {
			seen[lang] = true
		}
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// MarkersFor returns the discovery markers for the given languages
func MarkersFor(langs ...string) []string {
	want := toSet(langs)
	seen := map[string]bool{}
	var out []string
	for _, s := range Registered() {
		m, ok := s.(Meta)
		if !ok {
			continue
		}
		if len(want) == 0 || want[m.Language()] {
			for _, marker := range m.Markers() {
				if !seen[marker] {
					seen[marker] = true
					out = append(out, marker)
				}
			}
		}
	}
	return out
}

// selectScanners returns the scanners to run for a language selection
func selectScanners(langs ...string) []Scanner {
	want := toSet(langs)
	var out []Scanner
	for _, s := range Registered() {
		lang := languageOf(s)
		if lang == "" || len(want) == 0 || want[lang] {
			out = append(out, s)
		}
	}
	return out
}

// Run executes the scanners for the given languages against fsys and returns collected Facts
func Run(fsys fs.FS, repo model.Repo, langs ...string) (*model.Facts, error) {
	facts := model.NewFacts(repo)
	var errs []error
	for _, s := range selectScanners(langs...) {
		if err := s.Scan(fsys, facts); err != nil {
			errs = append(errs, err)
		}
	}
	return facts, joinErrors(errs)
}

// toSet builds a set from a slice, ignoring empty strings.
func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, i := range items {
		if i != "" {
			set[i] = true
		}
	}
	return set
}
