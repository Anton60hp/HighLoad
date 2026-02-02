# IoT Stream Processor

Высоконагруженный сервис для обработки потоковых метрик IoT на Go с аналитикой, кэшированием в Redis, мониторингом в Prometheus и развертыванием в Kubernetes.

## Содержание

- [Архитектура](#архитектура)
- [Возможности](#возможности)
- [Структура проекта](#структура-проекта)
- [API Endpoints](#api-endpoints)
- [Быстрый старт](#быстрый-старт)
- [Развертывание в Kubernetes](#развертывание-в-kubernetes)
- [Мониторинг](#мониторинг)
- [Нагрузочное тестирование](#нагрузочное-тестирование)
- [Оптимизации производительности](#оптимизации-производительности)

## Архитектура

Сервис состоит из следующих компонентов:

- **HTTP API** - прием метрик от IoT устройств
- **Analytics Engine** - асинхронная обработка метрик с использованием goroutines и channels
- **Rolling Window** - расчет скользящего среднего (окно 50 событий)
- **Anomaly Detector** - детекция аномалий на основе Z-score (порог > 2σ)
- **Redis Cache** - кэширование результатов анализа (TTL 5 минут)
- **Prometheus Metrics** - экспорт метрик для мониторинга

### Диаграмма архитектуры

```
IoT Devices → HTTP API → Analytics Engine → Redis Cache
                                               ↓
                                       Prometheus Metrics
                                               ↓
                                         Kubernetes HPA
```

## Возможности

-  Обработка 1000+ RPS
-  Асинхронная обработка метрик через goroutines и channels
-  Скользящее среднее для сглаживания данных (окно 50 событий)
-  Детекция аномалий на основе Z-score (порог 2σ)
-  Кэширование результатов в Redis
-  Prometheus метрики для мониторинга
-  Горизонтальное автомасштабирование (HPA) в Kubernetes
-  Health checks и readiness probes
-  Graceful shutdown

## Структура проекта

```
.
├── main.go                    # Точка входа, настройка HTTP сервера
├── handlers/                  # HTTP обработчики
│   └── metric_handler.go      # Обработка POST /metric, GET /analyze
├── models/                    # Модели данных
│   └── metric.go              # Metric, AnalysisResult
├── analytics/                 # Модуль аналитики
│   ├── engine.go              # Движок обработки метрик
│   ├── rolling_window.go      # Скользящее окно
│   └── anomaly_detector.go    # Детектор аномалий
├── cache/                     # Кэширование
│   └── redis.go               # Redis клиент
├── tools/                     # Инструменты тестирования
│   └── loadtest.go            # Нагрузочный тест на Go
├── k8s/                       # Kubernetes манифесты
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── redis-deployment.yaml
│   ├── app-deployment.yaml
│   ├── hpa.yaml
│   └── ingress.yaml
├── Dockerfile                 # Docker образ (мультистейдж, < 300MB)
├── deploy.sh                  # Скрипт развертывания (Linux/macOS)
├── deploy.ps1                 # Скрипт развертывания (Windows)
└── README.md                  # Документация
```

## API Endpoints

### POST /metric
Принимает метрику от IoT устройства.

**Request:**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "device_id": "sensor-1",
  "cpu": 0.75,
  "rps": 150
}
```

**Response:**
```json
{
  "status": "accepted",
  "device_id": "sensor-1"
}
```

### GET /analyze?device_id=sensor-1
Возвращает последние вычисленные значения из Redis.

**Response:**
```json
{
  "device_id": "sensor-1",
  "rolling_average_rps": 145.5,
  "is_anomaly": false,
  "z_score": 1.2,
  "processed_at": "2024-01-15T10:30:00Z"
}
```

### GET /health
Проверка работоспособности сервиса.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### GET /metrics
Prometheus метрики:
- `http_requests_total{method, endpoint, status}` - общее количество HTTP запросов
- `request_duration_seconds{method, endpoint}` - гистограмма длительности запросов
- `anomalies_detected_total{device_id}` - количество обнаруженных аномалий

## Быстрый старт

### Требования
- Go 1.22+
- Docker (для Redis)
- Kubernetes кластер (Minikube/Kind) - для развертывания в K8s

### Локальный запуск

1. **Установите зависимости:**
```bash
go mod download
```

2. **Запустите Redis:**
```bash
docker run -d -p 6379:6379 --name redis redis:7-alpine
```

3. **Запустите сервис:**
```bash
go run main.go
```

Сервис будет доступен на `http://localhost:8080`

### Тестирование API

**Отправка тестовой метрики:**
```bash
curl -X POST http://localhost:8080/metric \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2024-01-15T10:30:00Z",
    "device_id": "sensor-1",
    "cpu": 0.75,
    "rps": 150
  }'
```

**Проверка анализа:**
```bash
curl http://localhost:8080/analyze?device_id=sensor-1
```

**Health check:**
```bash
curl http://localhost:8080/health
```

**Prometheus метрики:**
```bash
curl http://localhost:8080/metrics
```

## Развертывание в Kubernetes

### Подготовка кластера

**Minikube:**
```bash
minikube start --cpus=2 --memory=4g
minikube addons enable ingress
```

### Сборка Docker образа

**Для Minikube:**
```bash
eval $(minikube docker-env)
docker build -t iot-processor:latest .
eval $(minikube docker-env -u)
```

**Для других кластеров:**
```bash
docker build -t your-registry/iot-processor:latest .
docker push your-registry/iot-processor:latest
```

### Развертывание

**Автоматическое развертывание (Linux/macOS):**
```bash
chmod +x deploy.sh
./deploy.sh
```

**Автоматическое развертывание (Windows):**
```powershell
.\deploy.ps1
```

**Ручное развертывание:**
```bash
# 1. Создание namespace
kubectl apply -f k8s/namespace.yaml

# 2. Создание ConfigMap
kubectl apply -f k8s/configmap.yaml

# 3. Развертывание Redis
kubectl apply -f k8s/redis-deployment.yaml

# 4. Ожидание готовности Redis
kubectl wait --for=condition=ready pod -l app=redis -n iot-stream-processor --timeout=120s

# 5. Развертывание приложения
kubectl apply -f k8s/app-deployment.yaml

# 6. Создание HPA
kubectl apply -f k8s/hpa.yaml

# 7. Создание Ingress (опционально)
kubectl apply -f k8s/ingress.yaml
```

### Проверка развертывания

```bash
# Проверка подов
kubectl get pods -n iot-stream-processor

# Проверка сервисов
kubectl get svc -n iot-stream-processor

# Проверка HPA
kubectl get hpa -n iot-stream-processor

# Просмотр логов
kubectl logs -f -l app=iot-processor -n iot-stream-processor
```

### Доступ к сервису

**Port forwarding:**
```bash
kubectl port-forward -n iot-stream-processor svc/iot-processor-service 8080:80
```

**Через Ingress (если настроен):**
```bash
# Добавьте в /etc/hosts (Linux/macOS) или C:\Windows\System32\drivers\etc\hosts (Windows)
# <ingress-ip> iot-processor.local

curl http://iot-processor.local/health
```

### Горизонтальное автомасштабирование (HPA)

HPA настроен на основе CPU utilization:
- **Минимум реплик**: 2
- **Максимум реплик**: 5
- **Целевая CPU**: 70%


## Мониторинг

Prometheus и Grafana **автоматически устанавливаются** при развертывании через скрипты `deploy.sh` или `deploy.ps1`.

### Автоматическая установка

При запуске скрипта развертывания автоматически:
-  Устанавливается kube-prometheus-stack (Prometheus + Grafana + Alertmanager)
-  Создается ServiceMonitor для автоматического сбора метрик
-  Создается готовый Grafana дашборд "IoT Stream Processor"
-  Настраиваются правила алертов Prometheus

### Доступ к сервисам

**Prometheus:**
```bash
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090
# http://localhost:9090
```

**Grafana:**
```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80
# http://localhost:3000
# Login: admin
# Password: admin
```

### Проверка работы мониторинга

1. **Проверка ServiceMonitor:**
```bash
kubectl get servicemonitor -n iot-stream-processor
```

2. **Проверка targets в Prometheus:**
   - Откройте Prometheus UI → Status → Targets
   - Должен быть target `iot-processor` в состоянии UP

3. **Проверка дашборда в Grafana:**
   - Откройте Grafana UI → Dashboards → Browse
   - Должен быть дашборд "IoT Stream Processor"




## Нагрузочное тестирование

### Использование встроенного loadtest

**Базовый тест:**
```bash
go run tools/loadtest.go http://localhost:8080/metric
```

**С параметрами:**
```bash
# Легкая нагрузка (100 RPS)
go run tools/loadtest.go http://localhost:8080/metric 4 100 30s

# Средняя нагрузка (500 RPS)
go run tools/loadtest.go http://localhost:8080/metric 8 200 30s

# Высокая нагрузка (1000+ RPS)
go run tools/loadtest.go http://localhost:8080/metric 12 500 60s
```

**Параметры:**
- `threads` - количество потоков
- `connections` - количество одновременных соединений
- `duration` - длительность теста (например, 30s, 60s, 2m)


### Ожидаемые результаты

При нагрузке 1000 RPS:
- **Success Rate**: > 95%
- **Latency (p50)**: < 20ms
- **Latency (p95)**: < 50ms
- **Latency (p99)**: < 100ms
- **Error rate**: < 0.1%

### Тестирование в Kubernetes c HPA

```bash
# Port forward
kubectl port-forward -n iot-stream-processor svc/iot-processor-service 8080:80

# Запуск теста
go run tools/loadtest.go http://localhost:8080/metric 12 500 60s

# Мониторинг масштабирования
watch kubectl get hpa,pods -n iot-stream-processor
```

