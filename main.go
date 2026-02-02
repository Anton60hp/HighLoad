package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"iot-stream-processor/cache"
	"iot-stream-processor/handlers"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Получение адреса Redis из переменной окружения или значение по умолчанию
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Инициализация Redis клиента
	redisClient, err := cache.NewRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Redis at %s: %v", redisAddr, err)
	}
	defer redisClient.Close()
	log.Printf("Connected to Redis at %s", redisAddr)

	// Создание роутера
	r := mux.NewRouter()

	// Инициализация обработчиков
	metricHandler := handlers.NewMetricHandler(redisClient)

	// Регистрация маршрутов
	r.HandleFunc("/health", handlers.HealthCheck).Methods("GET")
	r.HandleFunc("/metric", metricHandler.HandleMetric).Methods("POST")
	r.HandleFunc("/analyze", metricHandler.HandleAnalyze).Methods("GET")

	// Prometheus метрики
	r.Path("/metrics").Handler(promhttp.Handler())

	// Настройка HTTP сервера с оптимизацией для высокой нагрузки
	srv := &http.Server{
		Addr:           ":8080",
		Handler:        r,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Graceful shutdown
	go func() {
		log.Println("Server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Ожидание сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
