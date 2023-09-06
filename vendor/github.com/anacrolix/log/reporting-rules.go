package log

import (
	"fmt"
	"os"
	"strings"
)

var rules []Rule

type Rule func(names []string) (level Level, matched bool)

func alwaysLevel(level Level) Rule {
	return func(names []string) (Level, bool) {
		return level, true
	}
}

func stringSliceContains(s string, ss []string) bool {
	for _, sss := range ss {
		if strings.Contains(sss, s) {
			return true
		}
	}
	return false
}

func containsAllNames(all []string, level Level) Rule {
	return func(names []string) (_ Level, matched bool) {
		for _, s := range all {
			//log.Println(s, all, names)
			if !stringSliceContains(s, names) {
				return
			}
		}
		return level, true
	}
}

func parseRuleString(s string) (_ Rule, ok bool, _ error) {
	if s == "" {
		return
	}
	ss := strings.SplitN(s, "=", 2)
	level := NotSet
	var names []string
	if ss[0] != "*" {
		names = []string{ss[0]}
	}
	if len(ss) > 1 {
		var ok bool
		var err error
		level, ok, err = levelFromString(ss[1])
		if !ok {
			// blah= means disable the name, but just blah means to always include it
			level = disabled
		}
		if err != nil {
			return nil, false, fmt.Errorf("parsing level %q: %w", ss[1], err)
		}
	}
	return containsAllNames(names, level), true, nil
}

func parseEnvRules() (rules []Rule, err error) {
	rulesStr := os.Getenv("GO_LOG")
	ruleStrs := strings.Split(rulesStr, ",")
	for _, ruleStr := range ruleStrs {
		rule, ok, err := parseRuleString(ruleStr)
		if err != nil {
			return nil, fmt.Errorf("parsing rule %q: %w", ruleStr, err)
		}
		if !ok {
			continue
		}
		rules = append(rules, rule)
	}
	return
}

func levelFromString(s string) (level Level, ok bool, err error) {
	if s == "" {
		return
	}
	ok = true
	err = level.UnmarshalText([]byte(s))
	return
}

func levelFromRules(names []string) (level Level, ok bool) {
	defer func() {
		reportLevelFromRules(level, ok, names)
	}()
	// Later rules take precedence, so work backwards
	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		level, ok = r(names)
		if ok {
			return
		}
	}
	return
}
