package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	requestCount  int64
	successCount  int64
	failCount     int64
	totalLatency  int64 // в наносекундах
	minLatency    int64 = 1 << 62
	maxLatency    int64
	latencies     []int64
	latenciesLock sync.Mutex
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run tools/loadtest.go <url> [threads] [connections] [duration]")
		fmt.Println("Example: go run tools/loadtest.go http://localhost:8080/metric 4 100 30s")
		os.Exit(1)
	}

	url := os.Args[1]
	threads := 4
	connections := 100
	duration := 30 * time.Second

	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[2], "%d", &threads)
	}
	if len(os.Args) > 3 {
		fmt.Sscanf(os.Args[3], "%d", &connections)
	}
	if len(os.Args) > 4 {
		d, err := time.ParseDuration(os.Args[4])
		if err == nil {
			duration = d
		}
	}

	fmt.Printf("Load Test Configuration:\n")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Threads: %d\n", threads)
	fmt.Printf("  Connections: %d\n", connections)
	fmt.Printf("  Duration: %v\n\n", duration)

	// Инициализация
	latencies = make([]int64, 0, 10000)
	startTime := time.Now()
	endTime := startTime.Add(duration)

	// Запуск worker'ов
	var wg sync.WaitGroup
	workersPerThread := connections / threads
	if workersPerThread == 0 {
		workersPerThread = 1
	}

	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(url, workersPerThread, endTime)
		}()
	}

	// Ожидание завершения
	wg.Wait()
	totalDuration := time.Since(startTime)

	// Вычисление статистики
	printResults(totalDuration)
}

func worker(url string, connections int, endTime time.Time) {
	// Оптимизированный HTTP клиент с connection pooling
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Отправляем запросы без задержек для максимальной нагрузки
	for time.Now().Before(endTime) {
		sendRequest(client, url)
	}
}

func sendRequest(client *http.Client, url string) {
	// Генерация тестовой метрики
	metric := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"device_id": fmt.Sprintf("sensor-%d", time.Now().UnixNano()%10),
		"cpu":       0.5 + (float64(time.Now().UnixNano()%100) / 200.0),
		"rps":       100 + int(time.Now().UnixNano()%400),
	}

	jsonData, _ := json.Marshal(metric)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	atomic.AddInt64(&requestCount, 1)

	if err != nil || resp.StatusCode != http.StatusOK {
		atomic.AddInt64(&failCount, 1)
		if resp != nil {
			resp.Body.Close()
		}
		return
	}

	atomic.AddInt64(&successCount, 1)
	resp.Body.Close()

	// Обновление статистики задержек
	latencyNs := latency.Nanoseconds()
	atomic.AddInt64(&totalLatency, latencyNs)

	for {
		oldMin := atomic.LoadInt64(&minLatency)
		if latencyNs >= oldMin {
			break
		}
		if atomic.CompareAndSwapInt64(&minLatency, oldMin, latencyNs) {
			break
		}
	}

	for {
		oldMax := atomic.LoadInt64(&maxLatency)
		if latencyNs <= oldMax {
			break
		}
		if atomic.CompareAndSwapInt64(&maxLatency, oldMax, latencyNs) {
			break
		}
	}

	latenciesLock.Lock()
	latencies = append(latencies, latencyNs)
	latenciesLock.Unlock()
}

func printResults(duration time.Duration) {
	total := atomic.LoadInt64(&requestCount)
	success := atomic.LoadInt64(&successCount)
	failed := atomic.LoadInt64(&failCount)
	totalLat := atomic.LoadInt64(&totalLatency)
	minLat := atomic.LoadInt64(&minLatency)
	maxLat := atomic.LoadInt64(&maxLatency)

	avgLatency := time.Duration(0)
	if success > 0 {
		avgLatency = time.Duration(totalLat / success)
	}

	// Вычисление перцентилей
	latenciesLock.Lock()
	latenciesCopy := make([]int64, len(latencies))
	copy(latenciesCopy, latencies)
	latenciesLock.Unlock()

	var p50, p95, p99 time.Duration
	if len(latenciesCopy) > 0 {
		// Сортировка для перцентилей
		sort.Slice(latenciesCopy, func(i, j int) bool {
			return latenciesCopy[i] < latenciesCopy[j]
		})

		p50Idx := len(latenciesCopy) * 50 / 100
		p95Idx := len(latenciesCopy) * 95 / 100
		p99Idx := len(latenciesCopy) * 99 / 100

		if p50Idx < len(latenciesCopy) {
			p50 = time.Duration(latenciesCopy[p50Idx])
		}
		if p95Idx < len(latenciesCopy) {
			p95 = time.Duration(latenciesCopy[p95Idx])
		}
		if p99Idx < len(latenciesCopy) {
			p99 = time.Duration(latenciesCopy[p99Idx])
		}
	}

	rps := float64(total) / duration.Seconds()

	fmt.Println("\n==========================================")
	fmt.Println("Load Test Results")
	fmt.Println("==========================================")
	fmt.Printf("Duration:        %v\n", duration)
	fmt.Printf("Total Requests: %d\n", total)
	fmt.Printf("Successful:     %d\n", success)
	fmt.Printf("Failed:         %d\n", failed)
	fmt.Printf("Success Rate:   %.2f%%\n", float64(success)/float64(total)*100)
	fmt.Printf("Requests/sec:   %.2f\n", rps)
	fmt.Println("\nLatency Statistics:")
	fmt.Printf("  Min:          %v\n", time.Duration(minLat))
	fmt.Printf("  Max:          %v\n", time.Duration(maxLat))
	fmt.Printf("  Average:      %v\n", avgLatency)
	if p50 > 0 {
		fmt.Printf("  p50:          %v\n", p50)
	}
	if p95 > 0 {
		fmt.Printf("  p95:          %v\n", p95)
	}
	if p99 > 0 {
		fmt.Printf("  p99:          %v\n", p99)
	}
	fmt.Println("==========================================")
}
