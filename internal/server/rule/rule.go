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
}

func New(pattern string, ignoreAuthorizationHeader bool, ignoreParameters []string) (Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Rule{}, fmt.Errorf("failed to parse regular expression for path pattern %s: %w",
			pattern, err)
	}

	return Rule{
		re:                        re,
		ignoreAuthorizationHeader: ignoreAuthorizationHeader,
		ignoreParameters:          ignoreParameters,
	}, nil
}

func (rules Rules) IgnoreAuthorizationHeader(url string) bool {
	for _, rule := range rules {
		if !rule.re.MatchString(url) {
			continue
		}

		return rule.ignoreAuthorizationHeader
	}

	return false
}

func (rules Rules) IgnoredParamters(url string) []string {
	for _, rule := range rules {
		if !rule.re.MatchString(url) {
			continue
		}

		return rule.ignoreParameters
	}

	return []string{}
}
