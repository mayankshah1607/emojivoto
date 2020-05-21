package cache

import (
	"encoding/json"
	"log"
	"time"

	"github.com/go-redis/redis"
)

var (
	client *redis.Client
	expiry time.Duration
)

// InitCache initializes the cache
func InitCache(host, password string, db int, exp time.Duration) error {
	client = redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
		DB:       db,
	})
	expiry = exp
	_, err := client.Ping().Result()
	if err != nil {
		return err
	}
	log.Println("Successfully initialized redis client")
	return nil
}

// Set sets an emoji on the cache
func Set(key string, value *Emoji) error {
	json, err := json.Marshal(value)
	if err != nil {
		return err
	}

	client.Set(key, json, expiry*time.Second)
	log.Printf("Successfully cached emoji %s\n", key)

	return nil
}

// Get gets an emoji from the cache
func Get(key string) (*Emoji, error) {
	emoji := Emoji{}

	val, err := client.Get(key).Result()
	if err != nil {
		return &emoji, err
	}

	err = json.Unmarshal([]byte(val), &emoji)
	if err != nil {
		return &emoji, err
	}

	return &emoji, nil
}
