package main

import (
	"errors"
	"github.com/spf13/viper"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func ReadUserIP(r *http.Request) string {
	IPAddress := r.RemoteAddr
	for _, h := range []string{"X-Forwarded-For", "X-Real-Ip"} {
		addresses := strings.Split(r.Header.Get(h), ",")
		// march from right to left until we get a public address
		// that will be the address right before our proxy.
		for i := len(addresses) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(addresses[i])
			// header can contain spaces too, strip those out.
			realIP := net.ParseIP(ip)
			if !realIP.IsGlobalUnicast() {
				// bad address, go to next
				continue
			}
			IPAddress = ip
		}
	}

	if debugFlag {
		// Don't ignore port in debug so requests appear from different origins
		return strings.Replace(IPAddress, ":", "", -1)
	}

	host, _, _ := net.SplitHostPort(IPAddress)
	return host
}

type methods struct {
	values []string
}

func (m *methods) Contains(x string) bool {
	for _, n := range m.values {
		if x == n {
			return true
		}
	}
	return false
}

type LimiterRule struct {
	name            string
	pathMatch       *regexp.Regexp
	ipMatch         *net.IP
	cidrMatch       *net.IPNet
	httpMethodMatch methods
	limit           int64
	window          time.Duration
	rate            float64
	burst           int64
}

func (r *LimiterRule) getKey(req *http.Request) string {
	key := []string{"", "", ""}
	if r.cidrMatch != nil {
		key[0] = r.cidrMatch.String()
	} else {
		key[0] = ReadUserIP(req)
	}

	if len(r.httpMethodMatch.values) > 0 {
		key[1] = req.Method
	}

	if r.pathMatch != nil {
		key[2] = r.pathMatch.String()
	}

	return strings.Join(key, ":")
}

func newLimiterRules(literals []LimiterRuleLiteral) ([]LimiterRule, error) {
	rules := make([]LimiterRule, len(literals))
	for i, l := range literals {
		var cidrMatch *net.IPNet
		if l.CidrMatch != "" {
			_, cidrMatch, _ = net.ParseCIDR(l.CidrMatch)
		}

		var regex *regexp.Regexp
		if l.PathMatch != "" {
			regex, _ = regexp.Compile(l.PathMatch)
		}

		ip := net.ParseIP(l.IpMatch)

		if l.Limit <= 0 {
			return nil, errors.New("limit must be greater than 0")
		}

		duration, err := time.ParseDuration(l.Window)
		if err != nil {
			return nil, errors.New("invalid window duration")
		}
		rate := 1 / (float64(l.Limit) / duration.Seconds())

		rules[i] = LimiterRule{
			name:            l.Name,
			pathMatch:       regex,
			ipMatch:         &ip,
			cidrMatch:       cidrMatch,
			httpMethodMatch: methods{l.HttpMethodMatch},
			limit:           l.Limit,
			window:          duration,
			rate:            rate,
			burst:           l.Burst,
		}
	}

	return rules, nil
}

type LimiterRuleLiteral struct {
	Name            string
	PathMatch       string
	IpMatch         string
	CidrMatch       string
	HttpMethodMatch []string
	Limit           int64
	Window          string
	Burst           int64
}

func (r *LimiterRule) matchRequest(request *http.Request) bool {
	if len(r.httpMethodMatch.values) > 0 && !r.httpMethodMatch.Contains(request.Method) {
		return false
	}

	ip := net.ParseIP(ReadUserIP(request))

	if *r.ipMatch != nil && !r.ipMatch.Equal(ip) {
		return false
	}

	if r.cidrMatch != nil && !r.cidrMatch.Contains(ip) {
		return false
	}

	if r.pathMatch != nil && !r.pathMatch.MatchString(request.RequestURI) {
		return false
	}

	return true
}

type Rules struct {
	rules []LimiterRule
}

func NewRules(rules []LimiterRuleLiteral) (*Rules, error) {
	limiterRules, err := newLimiterRules(rules)
	if err != nil {
		return nil, err
	}

	return &Rules{rules: limiterRules}, nil
}

func (l *Rules) matchRequest(request *http.Request) (*LimiterRule, error) {
	for _, rule := range l.rules {
		if matches := rule.matchRequest(request); matches {
			return &rule, nil
		}
	}

	return nil, errors.New("no match")
}

func buildRulesFromConfig() (*Rules, error) {
	var literals []LimiterRuleLiteral
	err := viper.UnmarshalKey("rules", &literals)
	if err != nil {
		log.Println("invalid rules")
		return nil, err
	}

	rules, err := NewRules(literals)
	if err != nil {
		log.Println("invalid rules")
		return nil, err
	}

	return rules, nil
}
