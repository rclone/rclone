package fshttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/rclone/rclone/fs"
)

type frontingRule struct {
	raw      string
	suffix   string
	isSuffix bool
}

func (r frontingRule) matches(host string) bool {
	if r.isSuffix {
		return len(host) > len(r.suffix) && strings.HasSuffix(host, r.suffix)
	}
	return host == r.raw
}

type frontingConfig struct {
	targetHost string
	sniHost    string
	rules      []frontingRule
}

type frontingSNIContextKeyType struct{}

var frontingSNIContextKey = frontingSNIContextKeyType{}

func parseFrontingConfig(ci *fs.ConfigInfo) (*frontingConfig, error) {
	if !ci.FrontingEnable {
		return nil, nil
	}

	targetHost, err := parseFrontingTarget(ci.FrontingTarget)
	if err != nil {
		return nil, err
	}

	rules, err := parseFrontingRules(ci.FrontingDomains)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("fronting: no valid --fronting-domains rules configured")
	}

	sniHost := normalizeHost(ci.FrontingSNI)
	if sniHost == "" {
		sniHost = targetHost
	} else if strings.Contains(sniHost, ":") {
		return nil, fmt.Errorf("fronting: --fronting-sni must be a hostname only (no :port), got %q", ci.FrontingSNI)
	}

	return &frontingConfig{
		targetHost: targetHost,
		sniHost:    sniHost,
		rules:      rules,
	}, nil
}

func parseFrontingTarget(input string) (host string, err error) {
	in := strings.TrimSpace(input)
	if in == "" {
		return "", fmt.Errorf("fronting: empty --fronting-target")
	}

	if strings.Contains(in, ":") {
		return "", fmt.Errorf("fronting: --fronting-target must be a hostname only (no :port), got %q", input)
	}
	host = normalizeHost(in)
	if host == "" {
		return "", fmt.Errorf("fronting: invalid --fronting-target %q", input)
	}
	return host, nil
}

func parseFrontingRules(inputs []string) ([]frontingRule, error) {
	rawSource := strings.Join(inputs, ",")
	parts := strings.Split(rawSource, ",")
	rules := make([]frontingRule, 0, len(parts))
	for _, part := range parts {
		rule := normalizeHost(part)
		if rule == "" {
			continue
		}
		if rule == "*" || strings.Contains(rule, "*") && !strings.HasPrefix(rule, "*.") {
			return nil, fmt.Errorf("fronting: unsupported wildcard rule %q (use exact host or wildcard like *.example.com)", part)
		}
		if strings.HasPrefix(rule, "*.") {
			suffix := rule[1:]
			if suffix == "." || len(suffix) < 3 {
				return nil, fmt.Errorf("fronting: invalid wildcard rule %q", part)
			}
			rules = append(rules, frontingRule{
				raw:      rule,
				suffix:   suffix,
				isSuffix: true,
			})
			continue
		}
		if strings.Contains(rule, "*") {
			return nil, fmt.Errorf("fronting: invalid rule %q", part)
		}
		rules = append(rules, frontingRule{raw: rule})
	}
	return rules, nil
}

func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimSuffix(h, ".")
	return h
}

func hostOnlyFromRequest(req *http.Request) string {
	host := req.URL.Hostname()
	if req.Host != "" {
		host = req.Host
	}
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	return normalizeHost(host)
}

func requestPort(req *http.Request) string {
	if req.URL != nil {
		if port := req.URL.Port(); port != "" {
			return port
		}
		switch strings.ToLower(req.URL.Scheme) {
		case "http":
			return "80"
		case "https":
			return "443"
		}
	}
	return "443"
}

func withFrontingSNI(ctx context.Context, sni string) context.Context {
	return context.WithValue(ctx, frontingSNIContextKey, sni)
}

func frontingSNIFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	sni, ok := ctx.Value(frontingSNIContextKey).(string)
	return sni, ok && sni != ""
}

func (fc *frontingConfig) matchRule(host string) string {
	if fc == nil {
		return ""
	}
	for _, rule := range fc.rules {
		if rule.matches(host) {
			return rule.raw
		}
	}
	return ""
}
