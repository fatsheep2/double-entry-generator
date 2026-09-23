package cmd

import "testing"

func TestRuleFileProtocolValidatedBeforeMerge(t *testing.T) {
	for _, input := range []string{
		"protocolVersion: future/999\npersonalRules: []\n",
		"requiredCapabilities: [actions.teleport]\ntemplateRules: []\n",
	} {
		if _, err := parseRuleBytes([]byte(input)); err == nil {
			t.Errorf("incompatible rule file accepted before merge: %s", input)
		}
	}
	for _, input := range []string{
		"personalRules: []\n",
		"protocolVersion: mirato-deg-rules/1\nrequiredCapabilities: [actions.link]\npersonalRules: []\n",
	} {
		if _, err := parseRuleBytes([]byte(input)); err != nil {
			t.Errorf("supported rule file rejected: %v", err)
		}
	}
}
