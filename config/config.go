package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	common "github.com/ncabatoff/process-exporter"
	"gopkg.in/yaml.v2"
)

type (
	Config struct {
		Matchers []Matcher `yaml:"matchers"`
	}
	Matcher struct {
		Name    string `yaml:"name"`
		Comm    string `yaml:"comm"`
		User    string `yaml:"user,omitempty"`
		Ppid    int    `yaml:"ppid,omitempty"`
		Cmdline string `yaml:"cmdline,omitempty"`
	}
)

func (c *Config) String() string {
	return "config"
}

func (m *Matcher) String() string {
	return fmt.Sprintf("%v", m)
}

func (c *Config) MatchAndName(nacl common.ProcAttributes) (bool, string) {
	for _, m := range c.Matchers {
		if matched, name := m.MatchAndName(nacl); matched {
			return true, name
		}
	}
	return false, ""
}

func (m *Matcher) MatchAndName(nacl common.ProcAttributes) (bool, string) {
	if m.Comm != "" && m.Comm != nacl.Name {
		return false, m.Name
	}
	if m.User != "" && m.User != nacl.Username {
		return false, m.Name
	}
	if m.Ppid != 0 && m.Ppid != nacl.PPID {
		return false, m.Name
	}
	if m.Cmdline != "" && !strings.Contains(strings.Join(nacl.Cmdline, " "), m.Cmdline) {
		return false, m.Name
	}
	log.Printf("[%s] matched proc with [pid: %v, comm: %s, user: %s, ppid: %v, cmdline: %v]",
		m.Name, nacl.PID, nacl.Name, nacl.Username, nacl.PPID, nacl.Cmdline)
	return true, m.Name
}

// ReadRecipesFile opens the named file and extracts recipes from it.
func ReadFile(cfgpath string, debug bool) (*Config, error) {
	content, err := ioutil.ReadFile(cfgpath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file %q: %v", cfgpath, err)
	}
	if debug {
		log.Printf("Config file %q contents:\n%s", cfgpath, content)
	}
	return GetConfig(string(content), debug)
}

// GetConfig extracts Config from content by parsing it as YAML.
func GetConfig(content string, debug bool) (*Config, error) {
	var cfg Config
	err := yaml.Unmarshal([]byte(content), &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
