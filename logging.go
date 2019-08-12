package main

import (
	"github.com/fluent/fluent-logger-golang/fluent"
	"log"
	"net/http"
	"time"
)

type HttpConnection struct {
	Request    *http.Request
	Response   *http.Response
	Rule       *LimiterRule
	StatusCode int
}
type statsTransport struct {
	resp chan stats
}
type LogChannel chan *HttpConnection

var logChannel = make(LogChannel)
var popStats = make(chan statsTransport)

func LogRequest(fluent *fluent.Fluent, s stats) {
	tag := "meli-proxy.stats"

	for k, v := range s.counters {
		data := map[string]interface{}{
			"date":    s.time.Format(time.RFC3339),
			"rule":    k,
			"limited": v.limited,
			"allowed": v.allowed,
		}
		err := fluent.Post(tag, data)
		if err != nil {
			log.Println(err)
		}
	}
}

type reqs struct {
	limited uint
	allowed uint
}
type stats struct {
	time     time.Time
	counters map[string]*reqs
}

func startLogChannel(logger *fluent.Fluent) {
	go func() {
		var counter = make(map[string]*reqs)

		for {
			select {
			case read := <-popStats:
				read.resp <- stats{
					time:     time.Now(),
					counters: counter,
				}
				// Clear counter
				counter = make(map[string]*reqs)

			case conn := <-logChannel:
				// Save info
				if counter[conn.Rule.name] == nil {
					counter[conn.Rule.name] = &reqs{}
				}
				if conn.StatusCode == http.StatusTooManyRequests {
					counter[conn.Rule.name].limited++
				} else {
					counter[conn.Rule.name].allowed++
				}
			}
		}
	}()

	// Report every 1s
	ticker := time.NewTicker(1000 * time.Millisecond)
	go func() {
		read := statsTransport{
			resp: make(chan stats),
		}
		defer close(read.resp)

		for range ticker.C {
			popStats <- read
			stats := <-read.resp
			LogRequest(logger, stats)
		}
	}()
}
