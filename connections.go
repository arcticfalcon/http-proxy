package main

import (
	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/go-redis/redis"
	"log"
)

func newRedisClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", //ToDo: cluster config
		Password: "",
		DB:       0,
		PoolSize: 1000,
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.Println("WARNING: fail to initialize redis client: ", err)
	}

	return client, err
}

func newFluentdClient() (*fluent.Fluent, error) {
	logger, err := fluent.New(fluent.Config{})
	if err != nil {
		log.Println("WARNING: fail to initialize fluentd client: ", err)
	}
	defer logger.Close()

	return logger, err
}
