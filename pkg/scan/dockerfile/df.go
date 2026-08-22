// Package dockerfile scans Dockerfiles for base-image and stage facts.
package dockerfile

import (
	"bufio"
	"io/fs"
	"strings"

	"github.com/whoisnjoguu/elostirion/pkg/model"
	"github.com/whoisnjoguu/elostirion/pkg/scan"
)

func init() { scan.Register(Scanner{}) }

// Scanner extracts facts from a repository's Dockerfile.
type Scanner struct{}

// Name returns the fact namespace.
func (Scanner) Name() string { return "dockerfile" }

// Language is empty
func (Scanner) Language() string { return "" }

// Markers let a Dockerfile-only repository be discovered when no language filter is applied.
func (Scanner) Markers() []string { return []string{"Dockerfile"} }

// candidatePaths lists the Dockerfile locations checked, in priority order.
var candidatePaths = []string{"Dockerfile", "docker/Dockerfile", "build/Dockerfile"}

// Scan reads the Dockerfile and records base image facts
func (Scanner) Scan(fsys fs.FS, facts *model.Facts) error {
	path := ""
	for _, p := range candidatePaths {
		if f, err := fsys.Open(p); err == nil {
			_ = f.Close()
			path = p
			break
		}
	}
	if path == "" {
		return nil
	}
	f, err := fsys.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	// Track the base image of each named stage so a later "FROM builder" resolves
	// to a concrete image, and remember the last FROM as the final stage.
	stages := map[string]string{}
	var finalImage string
	stageCount := 0
	lastLine := 0

	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "FROM") {
			continue
		}
		image := fields[1]
		if resolved, ok := stages[image]; ok {
			image = resolved // FROM <previous-stage-name>
		}
		// Record the stage alias if present: FROM <image> AS <name>
		if len(fields) >= 4 && strings.EqualFold(fields[2], "AS") {
			stages[strings.ToLower(fields[3])] = image
		}
		finalImage = image
		stageCount++
		lastLine = line
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if finalImage == "" {
		return nil
	}

	loc := model.Location{File: path, Line: lastLine}
	facts.Set("dockerfile.base_image", finalImage, loc)
	facts.Set("dockerfile.final_image", finalImage, loc)
	facts.Set("dockerfile.stage_count", stageCount, model.Location{File: path})
	return nil
}
