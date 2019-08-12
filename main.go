package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/coocood/freecache"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var debugFlag bool
var port int
var proxyTo string
var cacheMb int

func main() {
	setupFlags()
	setupConfig()

	logger, _ := newFluentdClient()
	startLogChannel(logger)

	rules, err := buildRulesFromConfig()
	if err != nil {
		panic(err)
	}

	redisClient, _ := newRedisClient()
	proxy := &Proxy{
		rules:      rules,
		redisCache: &redisCache{client: redisClient},
		freecache:  &freeCache{fcache: freecache.NewCache(cacheMb * 1024 * 1024)},
	}

	// Update rules on config change
	viper.OnConfigChange(func(in fsnotify.Event) {
		rules, err := buildRulesFromConfig()
		if err != nil {
			log.Println("invalid rules")
			return
		}
		proxy.rules = rules
	})

	// Http server config
	srv := &http.Server{
		Handler:      proxy,
		Addr:         ":" + strconv.Itoa(port),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start Server
	go func() {
		log.Println("Starting Server")
		err = srv.ListenAndServe()
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()

	// Graceful Shutdown
	waitForShutdown(srv)
}

func setupFlags() {
	flag.BoolVar(&debugFlag, "debug", false, "run with debugging")
	flag.IntVar(&port, "port", 12345, "http port")
	flag.IntVar(&cacheMb, "cacheMb", 100, "megabytes for freecache")
	flag.StringVar(&proxyTo, "proxy", "http://localhost:8080", "URI to proxy to")
	flag.Parse()
}

func setupConfig() error {
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	// Find and read the config file
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
	viper.WatchConfig()
	return err
}

func waitForShutdown(srv *http.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Block until we receive our signal.
	<-interruptChan

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	srv.Shutdown(ctx)

	log.Println("Shutting down")
	os.Exit(0)
}
