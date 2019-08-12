package main

import (
	"io"
	"net/http"
	"strconv"
)

type Proxy struct {
	rules      *Rules
	redisCache *redisCache
	freecache  *freeCache
}

func (p *Proxy) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	rule, _ := p.rules.matchRequest(r)
	if rule != nil {
		key := rule.getKey(r)
		allowed, info, err := limit(p.freecache, rule, key, 1)
		if err != nil {
			// Use local limit
			allowed, info, err = limit(p.freecache, rule, key, 1)
			if err != nil {
				allowed = true
			}
		}

		if !allowed {
			h := wr.Header()
			h.Set("X-RateLimit-Limit", strconv.FormatInt(info.limit, 10))
			h.Set("X-RateLimit-Remaining", strconv.FormatInt(info.remaining, 10))
			h.Set("X-RateLimit-Reset", strconv.FormatInt(info.resetAfter, 10))
			http.Error(wr, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)

			// Send to log
			logChannel <- &HttpConnection{r, nil, rule, http.StatusTooManyRequests}

			return
		}
	}

	// Allowed
	var resp *http.Response
	var err error
	var req *http.Request
	client := &http.Client{}

	// Proxy request
	req, err = http.NewRequest(r.Method, proxyTo+r.RequestURI, r.Body)
	for name, value := range r.Header {
		req.Header.Set(name, value[0])
	}
	resp, err = client.Do(req)
	r.Body.Close()

	if err != nil {
		http.Error(wr, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers, status and body
	for k, v := range resp.Header {
		wr.Header().Set(k, v[0])
	}
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
	resp.Body.Close()

	// Send to log
	logChannel <- &HttpConnection{r, resp, rule, resp.StatusCode}
}
