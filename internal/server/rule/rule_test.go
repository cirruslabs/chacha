package rule_test

import (
	"github.com/cirruslabs/chacha/internal/server/rule"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewLineIsCounteredByUsingBeginningAndEnd(t *testing.T) {
	simpleRule, err := rule.New("^.*.good.com$",
		true, []string{"1", "2", "3"})
	require.NoError(t, err)

	rules := rule.Rules{simpleRule}

	require.True(t, rules.IgnoreAuthorizationHeader("subdomain.good.com"))
	require.Equal(t, []string{"1", "2", "3"}, rules.IgnoredParamters("subdomain.good.com"))

	require.False(t, rules.IgnoreAuthorizationHeader("evil.com\nsubdomain.good.com"))
	require.Empty(t, rules.IgnoredParamters("evil.com\nsubdomain.good.com"))
}

func TestFirstMatchWins(t *testing.T) {
	matchGranular, err := rule.New("https://cirrus-ci.com/task/[0-9]+",
		false, []string{"2"})
	require.NoError(t, err)

	matches := rule.Rules{matchGranular}
	require.False(t, matches.IgnoreAuthorizationHeader("https://cirrus-ci.com/task/123"))
	require.Equal(t, []string{"2"}, matches.IgnoredParamters("https://cirrus-ci.com/task/123"))

	matchCoarse, err := rule.New(`https://cirrus-ci.com/.*`,
		true, []string{"1"})
	require.NoError(t, err)

	matches = rule.Rules{matchCoarse, matchGranular}
	require.True(t, matches.IgnoreAuthorizationHeader("https://cirrus-ci.com/task/123"))
	require.Equal(t, []string{"1"}, matches.IgnoredParamters("https://cirrus-ci.com/task/123"))
}
