package kwir

import (
	"io/ioutil"
	"regexp"

	"gopkg.in/yaml.v2"
)

const (
	stopAfterFirstMatchPolicy = "StopAfterFirstMatch"
	applyAllRulesPolicy       = "ApplyAllRules"
)

type kwirConfig struct {
	RewritePolicy string       `yaml:"rewritePolicy,omitempty"`
	RewriteRules  rewriteRules `yaml:"rewriteRules"`
}

type rewriteRules struct {
	PrefixRules []rule `yaml:"prefixRules,omitempty"`
	RegexRules  []rule `yaml:"regexRules,omitempty"`
}

type rule struct {
	Match   string `yaml:"match"`
	Replace string `yaml:"replace"`

	regex *regexp.Regexp
}

func (config *kwirConfig) applyDefaults() {
	if config.RewriteRules.PrefixRules == nil {
		config.RewriteRules.PrefixRules = []rule{}
	}
	if config.RewriteRules.RegexRules == nil {
		config.RewriteRules.RegexRules = []rule{}
	}

	switch policy := config.RewritePolicy; policy {
	case stopAfterFirstMatchPolicy:
	case applyAllRulesPolicy:
	default:
		config.RewritePolicy = stopAfterFirstMatchPolicy
	}
}

func (config *kwirConfig) validateRegexpRules() error {
	for i, rl := range config.RewriteRules.RegexRules {
		exp, err := regexp.Compile(rl.Match)
		if err != nil {
			return err
		}

		config.RewriteRules.RegexRules[i].regex = exp
	}
	return nil
}

// parseKwirConfig returns the structured configuration parsed from a given file
func parseKwirConfig(filepath string) (kwirConfig, error) {
	fio, err := ioutil.ReadFile(filepath)
	if err != nil {
		return kwirConfig{}, err
	}

	var config kwirConfig
	if err := yaml.Unmarshal(fio, &config); err != nil {
		return kwirConfig{}, err
	}

	config.applyDefaults()

	err = config.validateRegexpRules()
	if err != nil {
		return kwirConfig{}, err
	}

	return config, nil
}
