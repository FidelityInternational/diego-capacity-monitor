package metrics

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry-community/go-cfenv"
	"gopkg.in/redis.v5"
	"time"
)

// MessageMetric - A struct of the firhose metrics we care about
type MessageMetric struct {
	Memory    float64 `json: "memory"`
	Timestamp int64   `json: "timestamp"`
}

// Metrics struct
type Metrics struct {
	MessageMetrics map[string]MessageMetric
	StaleDuration  time.Duration
	RedisClient    *redis.Client
}

// CreateMetrics - creates the "Metrics" control object
func CreateMetrics() Metrics {
	staleDuration := (15 * time.Minute)
	redisService, redisExists := redisServiceAvailable()
	if redisExists {
		redisClient, _ := createRedisClient(redisService)
		return Metrics{RedisClient: redisClient, StaleDuration: staleDuration}
	}
	messageMetrics := make(map[string]MessageMetric)
	return Metrics{MessageMetrics: messageMetrics, StaleDuration: staleDuration}
}

// GetAll - Gets all current metrics
func (m *Metrics) GetAll() map[string]MessageMetric {
	if m.RedisNotUsed() {
		return m.MessageMetrics
	}

	messageMetrics := make(map[string]MessageMetric)
	allKeys := m.RedisClient.Keys("*").Val()
	for _, key := range allKeys {
		messageMetrics[key] = m.redisGet(key)
	}
	return messageMetrics
}

// Delete - deletes the metric at the specified index
func (m *Metrics) Delete(index string) {
	if m.RedisNotUsed() {
		delete(m.MessageMetrics, index)
		return
	}
	m.RedisClient.Del(index)
}

// Set - sets the message metrics for the given index
func (m *Metrics) Set(index string, value MessageMetric) {
	if m.RedisNotUsed() {
		m.MessageMetrics[index] = value
		return
	}
	byteValue, _ := json.Marshal(value)
	m.RedisClient.Set(index, string(byteValue), 0)
}

// IsMetricStale - returns a bool based on the staleness of a metric
func (m *Metrics) IsMetricStale(index string) bool {
	var messageTimestamp int64
	if m.RedisNotUsed() {
		messageTimestamp = m.MessageMetrics[index].Timestamp
	} else {
		messageTimestamp = m.redisGet(index).Timestamp
	}
	return time.Now().After(time.Unix(0, messageTimestamp).Add(m.StaleDuration))
}

// ClearStaleMetrics - Deletes any metrics that are stale
func (m *Metrics) ClearStaleMetrics() {
	for index := range m.GetAll() {
		if m.IsMetricStale(index) {
			m.Delete(index)
		}
	}
}

func (m *Metrics) redisGet(key string) MessageMetric {
	var messageMetric MessageMetric
	messageMetricString := m.RedisClient.Get(key).Val()
	json.Unmarshal([]byte(messageMetricString), &messageMetric)
	return messageMetric
}

//RedisNotUsed - returns a bool for if redis is in use or not
func (m *Metrics) RedisNotUsed() bool {
	return (m.RedisClient == nil)
}

func redisServiceAvailable() (cfenv.Service, bool) {
	appEnv, err := cfenv.Current()
	if err != nil {
		fmt.Println("Could not get CF Env, assuming redis service does not exist")
		return cfenv.Service{}, false
	}
	services := appEnv.Services
	redisServices, err := services.WithTag("redis")
	if err != nil {
		fmt.Println("Could not get service tags, assuming redis service does not exist")
		return cfenv.Service{}, false
	}
	if len(redisServices) >= 1 {
		return redisServices[0], true
	}
	return cfenv.Service{}, false
}

func createRedisClient(redisService cfenv.Service) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%v", redisService.Credentials["host"], redisService.Credentials["port"]),
		Password: redisService.Credentials["password"].(string),
		DB:       0,
	})
	_, err := client.Ping().Result()
	if err != nil {
		fmt.Printf("Error connecting to Redis: %s", err.Error())
		return &redis.Client{}, err
	}
	return client, nil
}
