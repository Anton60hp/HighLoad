package cache

import (
	"context"
	"encoding/json"
	"iot-stream-processor/models"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisClient(addr string) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "", // без пароля
		DB:           0,  // используем DB по умолчанию
		PoolSize:     50, // Увеличенный пул соединений
		MinIdleConns: 10, // Минимальное количество idle соединений
		MaxRetries:   3,  // Количество попыток при ошибке
	})

	ctx := context.Background()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisClient{
		client: rdb,
		ctx:    ctx,
	}, nil
}

func (rc *RedisClient) Close() error {
	return rc.client.Close()
}

func (rc *RedisClient) SaveAnalysis(deviceID string, result models.AnalysisResult) error {
	key := "analysis:" + deviceID

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return rc.client.Set(rc.ctx, key, data, 5*time.Minute).Err()
}

func (rc *RedisClient) GetAnalysis(deviceID string) (*models.AnalysisResult, error) {
	key := "analysis:" + deviceID

	val, err := rc.client.Get(rc.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Результат не найден
	}
	if err != nil {
		return nil, err
	}

	var result models.AnalysisResult
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, err
	}

	return &result, nil
}
