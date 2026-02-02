package analytics

import (
	"iot-stream-processor/cache"
	"iot-stream-processor/models"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
)

type AnomalyCallback func(deviceID string)

type AnalyticsEngine struct {
	redisClient    *cache.RedisClient
	rollingWindows map[string]*RollingWindow
	anomalyDets    map[string]*AnomalyDetector
	mu             sync.RWMutex
	metricChan     chan models.Metric
	onAnomaly      AnomalyCallback
}

func NewAnalyticsEngine(redisClient *cache.RedisClient, onAnomaly AnomalyCallback) *AnalyticsEngine {
	engine := &AnalyticsEngine{
		redisClient:    redisClient,
		rollingWindows: make(map[string]*RollingWindow),
		anomalyDets:    make(map[string]*AnomalyDetector),
		metricChan:     make(chan models.Metric, 10000),
		onAnomaly:      onAnomaly,
	}

	numWorkers := runtime.NumCPU() * 2
	if envWorkers := os.Getenv("ANALYTICS_WORKERS"); envWorkers != "" {
		if w, err := strconv.Atoi(envWorkers); err == nil && w > 0 {
			numWorkers = w
		}
	}

	if numWorkers < 4 {
		numWorkers = 4
	}
	if numWorkers > 16 {
		numWorkers = 16
	}
	log.Printf("Starting %d analytics workers", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go engine.processMetrics()
	}

	return engine
}

func (ae *AnalyticsEngine) ProcessMetric(metric models.Metric) {
	select {
	case ae.metricChan <- metric:

	default:
		// Канал переполнен, логируем предупреждение
		log.Printf("WARNING: Metric channel is full, dropping metric from device %s", metric.DeviceID)
	}
}

func (ae *AnalyticsEngine) processMetrics() {
	for metric := range ae.metricChan {
		ae.processMetric(metric)
	}
}

func (ae *AnalyticsEngine) processMetric(metric models.Metric) {

	ae.mu.RLock()
	rollingWindow, exists := ae.rollingWindows[metric.DeviceID]
	anomalyDet, exists2 := ae.anomalyDets[metric.DeviceID]
	ae.mu.RUnlock()

	if !exists || !exists2 {
		ae.mu.Lock()

		if !exists {
			rollingWindow = NewRollingWindow(50)
			ae.rollingWindows[metric.DeviceID] = rollingWindow
		} else {
			rollingWindow = ae.rollingWindows[metric.DeviceID]
		}
		if !exists2 {
			anomalyDet = NewAnomalyDetector()
			ae.anomalyDets[metric.DeviceID] = anomalyDet
		} else {
			anomalyDet = ae.anomalyDets[metric.DeviceID]
		}
		ae.mu.Unlock()
	}

	rollingWindow.Add(metric.RPS)
	rollingAvg := rollingWindow.Average()

	isAnomaly, zScore := anomalyDet.Detect(metric.RPS)

	result := models.AnalysisResult{
		DeviceID:          metric.DeviceID,
		RollingAverageRPS: rollingAvg,
		IsAnomaly:         isAnomaly,
		ZScore:            zScore,
		ProcessedAt:       metric.ProcessedAt(),
	}

	go func(deviceID string, res models.AnalysisResult) {
		if err := ae.redisClient.SaveAnalysis(deviceID, res); err != nil {
			// log.Printf("ERROR: Failed to save analysis for device %s: %v", deviceID, err)
		}
	}(metric.DeviceID, result)

	if isAnomaly {
		log.Printf("ANOMALY DETECTED: device=%s, rps=%.2f, z_score=%.2f, rolling_avg=%.2f",
			metric.DeviceID, metric.RPS, zScore, rollingAvg)

		if ae.onAnomaly != nil {
			ae.onAnomaly(metric.DeviceID)
		}
	}
}
