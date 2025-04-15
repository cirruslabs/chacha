package rule_test

import (
	rulepkg "github.com/cirruslabs/chacha/internal/server/rule"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewLineIsCounteredByUsingBeginningAndEnd(t *testing.T) {
	simpleRule, err := rulepkg.New("^.*.good.com$",
		true, []string{"doesn't matter"}, false, false)
	require.NoError(t, err)

	rules := rulepkg.Rules{simpleRule}
	require.NotNil(t, rules.Get("subdomain.good.com"))
	require.Nil(t, rules.Get("evil.com\nsubdomain.good.com"))
}

func TestFirstMatchWins(t *testing.T) {
	matchGranular, err := rulepkg.New("https://cirrus-ci.com/task/[0-9]+",
		true, []string{"X-Granular"}, false, false)
	require.NoError(t, err)

	rules := rulepkg.Rules{matchGranular}
	rule := rules.Get("https://cirrus-ci.com/task/123")
	require.NotNil(t, rule)

	matchCoarse, err := rulepkg.New(`https://cirrus-ci.com/.*`,
		true, []string{"X-Coarse"}, false, false)
	require.NoError(t, err)
	require.Equal(t, []string{"X-Granular"}, rule.IgnoredParameters())

	rules = rulepkg.Rules{matchCoarse, matchGranular}
	rule = rules.Get("https://cirrus-ci.com/task/123")
	require.NotNil(t, rule)
	require.Equal(t, []string{"X-Coarse"}, rule.IgnoredParameters())
}
