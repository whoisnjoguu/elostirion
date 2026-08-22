package vcs

import "testing"

func TestParseRemote(t *testing.T) {
	cases := []struct {
		url                   string
		provider, owner, name string
	}{
		{"git@github.com:whoisnjoguu/elostirion.git", "github", "whoisnjoguu", "elostirion"},
		{"https://github.com/Prescott-Data/nexus-framework.git", "github", "Prescott-Data", "nexus-framework"},
		{"https://github.com/Prescott-Data/nexus-framework", "github", "Prescott-Data", "nexus-framework"},
		{"git@bitbucket.org:prescott/auth-api.git", "bitbucket", "prescott", "auth-api"},
		{"https://user@bitbucket.org/prescott/auth-api.git", "bitbucket", "prescott", "auth-api"},
		{"ssh://git@gitlab.com/group/proj.git", "gitlab", "group", "proj"},
	}
	for _, c := range cases {
		p, o, n, err := ParseRemote(c.url)
		if err != nil {
			t.Errorf("%s: %v", c.url, err)
			continue
		}
		if p != c.provider || o != c.owner || n != c.name {
			t.Errorf("%s: got %s/%s/%s want %s/%s/%s", c.url, p, o, n, c.provider, c.owner, c.name)
		}
	}
}

func TestParseRemoteRejectsGarbage(t *testing.T) {
	if _, _, _, err := ParseRemote("not a url"); err == nil {
		t.Fatal("expected error")
	}
}
