package python

import (
	"bufio"
	"io/fs"
	"strings"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

func init() { scan.Register(Scanner{}) }

// Scanner extracts facts from a Python repository
type Scanner struct{}

// Name returns the fact namespace.
func (Scanner) Name() string { return "python" }

// Language reports the language this scanner belongs to.
func (Scanner) Language() string { return "py" }

// Markers are files that identify a Python repository.
func (Scanner) Markers() []string {
	return []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"}
}

// Scan reads Python project metadata. Absence of Python files is not an error.
func (Scanner) Scan(fsys fs.FS, facts *model.Facts) error {
	facts.Set("python.package_manager", detectPackageManager(fsys), model.Location{})

	if data, ok := readFile(fsys, "pyproject.toml"); ok {
		loc := model.Location{File: "pyproject.toml"}
		if v := tomlValue(data, "requires-python"); v != "" {
			facts.Set("python.requires_version", v, loc)
		}
		if v := tomlValue(data, "name"); v != "" {
			facts.Set("python.project_name", v, loc)
		}
		return nil
	}

	if _, ok := readFile(fsys, "requirements.txt"); ok {
		// A requirements.txt alone still marks the repo as Python.
		facts.Set("python.requires_version", "", model.Location{File: "requirements.txt"})
	}
	return nil
}

// detectPackageManager infers the tool in use from lock and config files.
func detectPackageManager(fsys fs.FS) string {
	switch {
	case exists(fsys, "poetry.lock"):
		return "poetry"
	case exists(fsys, "uv.lock"):
		return "uv"
	case exists(fsys, "Pipfile.lock"), exists(fsys, "Pipfile"):
		return "pipenv"
	default:
		return "pip"
	}
}

// tomlValue extracts a top-level or [project] scalar string value for key from a pyproject.toml
func tomlValue(data []byte, key string) string {
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, key) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, key))
		if !strings.HasPrefix(rest, "=") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(rest, "="))
		val = strings.Trim(val, `"'`)
		return val
	}
	return ""
}

func readFile(fsys fs.FS, name string) ([]byte, bool) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return []byte(b.String()), true
}

func exists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
