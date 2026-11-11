// Package testutil contains opt-in guards for disposable integration fixtures.
package testutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"testing"
)

var ErrUnownedFixture = errors.New("owned business billing fixture required")

type BizFixtureService struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
type BizFixture struct {
	RunID    string                       `json:"runID"`
	Project  string                       `json:"project"`
	APIURL   string                       `json:"apiURL"`
	Services map[string]BizFixtureService `json:"services"`
}

func ValidateBizFixture() (BizFixture, error) {
	var m BizFixture
	raw, err := os.ReadFile(os.Getenv("FRN_BIZ_FIXTURE_MANIFEST"))
	if err != nil {
		return m, fmt.Errorf("%w: missing manifest", ErrUnownedFixture)
	}
	if json.Unmarshal(raw, &m) != nil || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(m.RunID) || m.Project != "frn-biz-"+m.RunID {
		return m, ErrUnownedFixture
	}
	for name, port := range map[string]string{"elasticsearch": "9200/tcp", "redis": "6379/tcp"} {
		s, ok := m.Services[name]
		u, e := url.Parse(s.URL)
		if !ok || e != nil || u.Hostname() != "127.0.0.1" || u.Port() == "" || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(s.ID) {
			return m, ErrUnownedFixture
		}
		data, e := exec.Command("docker", "inspect", s.ID).Output()
		if e != nil {
			return m, ErrUnownedFixture
		}
		var info []struct {
			ID              string `json:"Id"`
			Config          struct{ Labels map[string]string }
			NetworkSettings struct {
				Ports map[string][]struct {
					HostIP   string `json:"HostIp"`
					HostPort string
				}
			}
		}
		if json.Unmarshal(data, &info) != nil || len(info) != 1 || info[0].ID != s.ID || info[0].Config.Labels["com.freerangenotify.fixture.owner"] != m.RunID || info[0].Config.Labels["com.docker.compose.project"] != m.Project || info[0].Config.Labels["com.docker.compose.service"] != name {
			return m, ErrUnownedFixture
		}
		matched := false
		for _, p := range info[0].NetworkSettings.Ports[port] {
			if p.HostIP == "127.0.0.1" && p.HostPort == u.Port() {
				matched = true
			}
		}
		if !matched {
			return m, ErrUnownedFixture
		}
	}
	if configured := os.Getenv("INTEGRATION_ES_URL"); configured != "" && configured != m.Services["elasticsearch"].URL {
		return m, ErrUnownedFixture
	}
	if configured := os.Getenv("FRN_BIZ_TEST_ES_URL"); configured != "" && configured != m.Services["elasticsearch"].URL {
		return m, ErrUnownedFixture
	}
	return m, nil
}
func RequireOwnedBizFixture(t testing.TB) BizFixture {
	t.Helper()
	m, err := ValidateBizFixture()
	if err != nil {
		t.Fatal(err)
	}
	return m
}
