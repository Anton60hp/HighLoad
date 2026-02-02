package handlers

import (
	"encoding/json"
	"iot-stream-processor/analytics"
	"iot-stream-processor/cache"
	"iot-stream-processor/models"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	requestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	anomaliesDetectedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "anomalies_detected_total",
			Help: "Total number of anomalies detected",
		},
		[]string{"device_id"},
	)
)

type MetricHandler struct {
	redisClient *cache.RedisClient
	analytics   *analytics.AnalyticsEngine
}

func NewMetricHandler(redisClient *cache.RedisClient) *MetricHandler {

	onAnomaly := func(deviceID string) {
		anomaliesDetectedTotal.WithLabelValues(deviceID).Inc()
	}

	return &MetricHandler{
		redisClient: redisClient,
		analytics:   analytics.NewAnalyticsEngine(redisClient, onAnomaly),
	}
}

func (h *MetricHandler) HandleMetric(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	defer func() {
		duration := time.Since(start).Seconds()
		requestDurationSeconds.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	}()

	var metric models.Metric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	if err := metric.Validate(); err != nil {
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "400").Inc()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.analytics.ProcessMetric(metric)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "accepted",
		"device_id": metric.DeviceID,
	})

	httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "200").Inc()
}

func (h *MetricHandler) HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id parameter is required", http.StatusBadRequest)
		return
	}

	result, err := h.redisClient.GetAnalysis(deviceID)
	if err != nil {
		http.Error(w, "Failed to get analysis: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
