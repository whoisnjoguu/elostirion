package model

// Severity classifies how serious a Finding is
type Severity string

const (
	SeverityError Severity = "error" // blocks
	SeverityWarn  Severity = "warn"  // annotates but does not block by default
	SeverityDrift Severity = "drift" // marks a divergence detected against external state
	SeverityInfo  Severity = "info"  // advisory only.
)

// Rank returns an ordering for severities
func (s Severity) Rank() int {
	switch s {
	case SeverityError:
		return 3
	case SeverityDrift:
		return 2
	case SeverityWarn:
		return 1
	case SeverityInfo:
		return 0
	default:
		return 0
	}
}

// Repo identifies a single repository within a fleet.
type Repo struct {
	Provider string // TODO: consider making this an enum of known providers
	Owner    string
	Name     string
	Ref      string // branch, tag, or SHA the facts were read from
	Path     string // local filesystem path when Provider == "local"
}

// Slug returns the owner/name form of the repository.
func (r Repo) Slug() string {
	if r.Owner == "" {
		return r.Name
	}
	return r.Owner + "/" + r.Name
}

// Facts are the attributes a set of scanners extracted from one repository
type Facts struct {
	Repo      Repo
	Languages []string // detected in the repository by marker files
	Values    map[string]any
	Sources   map[string]Location
}

// HasLanguage reports whether lang was detected in the repository.
func (f *Facts) HasLanguage(lang string) bool {
	for _, l := range f.Languages {
		if l == lang {
			return true
		}
	}
	return false
}

// NewFacts returns an initialised Facts for a repository.
func NewFacts(repo Repo) *Facts {
	return &Facts{
		Repo:    repo,
		Values:  make(map[string]any),
		Sources: make(map[string]Location),
	}
}

// Set records a fact value and the file it was derived from.
func (f *Facts) Set(key string, value any, source Location) {
	f.Values[key] = value
	if source.File != "" {
		f.Sources[key] = source
	}
}

// Get returns a fact value and whether it was present.
func (f *Facts) Get(key string) (any, bool) {
	v, ok := f.Values[key]
	return v, ok
}

// Location points at a place in a file that a fact or finding refers to.
type Location struct {
	File string
	Line int // 1-based; 0 means whole file
}

// Finding is the result of evaluating one rule against one repo
type Finding struct {
	RuleID   string
	Message  string
	Severity Severity
	Repo     Repo
	Location Location
	Got      string
	Want     string
}

// ChangePlan is the set of file edits a recipe would make to converge a repo to spec
type ChangePlan struct {
	Repo    Repo
	Recipe  string
	Title   string
	Body    string
	Edits   []FileEdit
	Reasons []string // human-readable reasons
}

// Empty reports whether the plan would make no changes.
func (p ChangePlan) Empty() bool {
	return len(p.Edits) == 0
}

// FileEdit describes a single file mutation within a ChangePlan.
type FileEdit struct {
	Path    string
	Content []byte // full new content of the file
	Delete  bool   // when true, remove the file instead of writing Content
}
