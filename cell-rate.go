package main

import (
	"encoding/binary"
	"errors"
	"github.com/coocood/freecache"
	"github.com/go-redis/redis"
	"time"
)

const NANO = 1000000000
const MaxAttempts = 10

type RateLimitInfo struct {
	limit      int64
	remaining  int64
	resetAfter int64
}

type cache interface {
	get(key string) int64
	set(key string, v int64, exp int64) error
}

type redisCache struct {
	client *redis.Client
}

func (r *redisCache) get(key string) int64 {
	v, err := r.client.Get(key).Int64()
	if err != nil {
		return 0
	} else {
		return v
	}
}
func (r *redisCache) set(key string, v int64, exp int64) error {
	return r.client.Set(key, v, time.Duration(exp)).Err()
}

type freeCache struct {
	fcache *freecache.Cache
}

func (f *freeCache) get(key string) int64 {
	got, err := f.fcache.Get([]byte(key))
	if err != nil {
		return 0
	} else {
		return int64(binary.LittleEndian.Uint64(got))
	}
}

func (f *freeCache) set(key string, v int64, exp int64) error {
	var bKey [8]byte
	binary.LittleEndian.PutUint64(bKey[:], uint64(v))
	return f.fcache.Set([]byte(key), bKey[:], int(exp/NANO))
}

func limit(cache cache, rule *LimiterRule, key string, quantity int64) (bool, *RateLimitInfo, error) {
	rate := rule.rate
	burst := rule.burst

	emissionInterval := int64(rate * NANO) // Convert from seconds to nanoseconds.
	limit := burst + 1
	delayVariationTolerance := emissionInterval * limit

	i := 0
	for i < MaxAttempts {
		// tat refers to the theoretical arrival time that would be expected
		// from equally spaced requests at exactly the rate limit.
		tat := cache.get(key)
		now := time.Now().UnixNano()

		increment := quantity * emissionInterval

		// newTat describes the new theoretical arrival if the request would succeed.
		// If we get a `tat` in the past (empty bucket), use the current time instead. Having
		// a delayVariationTolerance >= 1 makes sure that at least one request with quantity 1 is
		// possible when the bucket is empty.
		newTat := now
		if tat > now {
			newTat = tat
		}
		newTat = newTat + increment

		allowAtAndAfter := newTat - delayVariationTolerance
		if now < allowAtAndAfter {

			info := RateLimitInfo{
				limit: limit,
				// Bucket size in duration minus time left until TAT, divided by the emission interval
				// to get a count
				// This is non-zero when a request with quantity > 1 is limited, but lower quantities
				// are still allowed.
				remaining: (delayVariationTolerance - (tat - now)) / emissionInterval,
				// Use `tat` instead of `newTat` - we don't further increment tat for a blocked request
				resetAfter: (tat - now) / NANO,
			}

			return false, &info, nil
		}

		ttl := newTat - now // Time until bucket is empty again

		err := cache.set(key, newTat, ttl) // ToDo: upgrade to CAS operation

		if err == nil {
			info := RateLimitInfo{
				limit:      limit,
				remaining:  (delayVariationTolerance - ttl) / emissionInterval,
				resetAfter: -1,
			}

			return true, &info, nil
		}

		// Retry
		i += 1
	}

	return false, nil, errors.New("failed to save limit")
}
