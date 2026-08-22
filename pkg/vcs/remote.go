package vcs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// hostProviders maps a remote host to a forge provider name.
var hostProviders = map[string]string{
	"github.com":    "github",
	"bitbucket.org": "bitbucket",
	"gitlab.com":    "gitlab",
}

// sshURL matches scp-like remotes
var sshURL = regexp.MustCompile(`^(?:[\w.-]+@)?([\w.-]+):([\w.-]+)/([\w.-]+?)(?:\.git)?$`)

// httpURL matches http(s)/ssh scheme remotes
var httpURL = regexp.MustCompile(`^(?:https?|ssh|git)://(?:[\w.-]+@)?([\w.-]+)(?::\d+)?/([\w.-]+)/([\w.-]+?)(?:\.git)?/?$`)

// RemoteSlug reads dir/.git/config and returns the provider, owner, and name of
// the origin remote.
func RemoteSlug(dir string) (provider, owner, name string, err error) {
	url, err := originURL(filepath.Join(dir, ".git", "config"))
	if err != nil {
		return "", "", "", err
	}
	return ParseRemote(url)
}

// ParseRemote extracts provider/owner/name from a git remote URL.
func ParseRemote(url string) (provider, owner, name string, err error) {
	var host string
	if m := httpURL.FindStringSubmatch(url); m != nil {
		host, owner, name = m[1], m[2], m[3]
	} else if m := sshURL.FindStringSubmatch(url); m != nil {
		host, owner, name = m[1], m[2], m[3]
	} else {
		return "", "", "", fmt.Errorf("vcs: unrecognized remote url %q", url)
	}
	p, ok := hostProviders[host]
	if !ok {
		p = host // self-hosted: caller decides what to do with it
	}
	return p, owner, name, nil
}

// originURL scans a .git/config for the [remote "origin"] url.
func originURL(configPath string) (string, error) {
	f, err := os.Open(configPath)
	if err != nil {
		return "", fmt.Errorf("vcs: %w", err)
	}
	defer f.Close()

	inOrigin := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(line, "url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("vcs: no origin remote in %s", configPath)
}
