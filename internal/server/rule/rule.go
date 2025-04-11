package rule

import (
	"fmt"
	"regexp"
)

type Rules []Rule

type Rule struct {
	re                        *regexp.Regexp
	ignoreAuthorizationHeader bool
	ignoreParameters          []string
	directConnect             bool
}

func New(pattern string, ignoreAuthorizationHeader bool, ignoreParameters []string, directConnect bool) (Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Rule{}, fmt.Errorf("failed to parse regular expression for path pattern %s: %w",
			pattern, err)
	}

	return Rule{
		re:                        re,
		ignoreAuthorizationHeader: ignoreAuthorizationHeader,
		ignoreParameters:          ignoreParameters,
		directConnect:             directConnect,
	}, nil
}

func (rule Rule) IgnoreAuthorizationHeader() bool {
	return rule.ignoreAuthorizationHeader
}

func (rule Rule) IgnoredParameters() []string {
	return rule.ignoreParameters
}

func (rule Rule) DirectConnect() bool {
	return rule.directConnect
}

func (rules Rules) Get(url string) *Rule {
	for _, rule := range rules {
		if rule.re.MatchString(url) {
			return &rule
		}
	}

	return nil
}
