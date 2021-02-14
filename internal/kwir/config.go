package kwir

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type kwirConfig struct {
	RewriteRules rewriteRules `yaml:"rewriteRules,omitempty"`
}

type rewriteRules struct {
	PrefixRules []rules `yaml:"prefixRules,omitempty"`
	RegexRules  []rules `yaml:"regexRules,omitempty"`
}

type rules struct {
	Match   string `yaml:"match"`
	Replace string `yaml:"replace"`
}

// parseKwirConfig returns the structured configuration parsed from a given file
func parseKwirConfig(conffile string) (kwirConfig, error) {
	fio, err := ioutil.ReadFile(conffile)
	if err != nil {
		return kwirConfig{}, fmt.Errorf("Unable to open config file")
	}

	var config kwirConfig
	if err := yaml.Unmarshal(fio, &config); err != nil {
		return kwirConfig{}, fmt.Errorf("Unable to unmarshal config file yaml")
	}

	return config, nil
}
