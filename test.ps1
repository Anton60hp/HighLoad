# PowerShell скрипт для тестирования API
# Использование: .\test.ps1

$baseUrl = "http://localhost:8080"

Write-Host "Testing IoT Stream Processor API" -ForegroundColor Green
Write-Host "================================" -ForegroundColor Green

# Тест 1: Health Check
Write-Host "`n1. Testing /health endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/health" -Method GET -UseBasicParsing
    Write-Host "Status: $($response.StatusCode)" -ForegroundColor Green
    Write-Host "Response: $($response.Content)"
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
}

# Тест 2: Отправка метрики
Write-Host "`n2. Testing POST /metric endpoint..." -ForegroundColor Yellow
$metric = @{
    timestamp = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
    device_id = "sensor-1"
    cpu = 0.75
    rps = 150
} | ConvertTo-Json

try {
    $response = Invoke-WebRequest -Uri "$baseUrl/metric" -Method POST -Body $metric -ContentType "application/json" -UseBasicParsing
    Write-Host "Status: $($response.StatusCode)" -ForegroundColor Green
    Write-Host "Response: $($response.Content)"
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
}

# Небольшая задержка для обработки
Start-Sleep -Seconds 1

# Тест 3: Получение анализа
Write-Host "`n3. Testing GET /analyze endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/analyze?device_id=sensor-1" -Method GET -UseBasicParsing
    Write-Host "Status: $($response.StatusCode)" -ForegroundColor Green
    Write-Host "Response: $($response.Content)"
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
}

# Тест 4: Prometheus метрики
Write-Host "`n4. Testing GET /metrics endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "$baseUrl/metrics" -Method GET -UseBasicParsing
    Write-Host "Status: $($response.StatusCode)" -ForegroundColor Green
    Write-Host "Metrics (first 500 chars):"
    Write-Host $response.Content.Substring(0, [Math]::Min(500, $response.Content.Length))
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
}

Write-Host "`n================================" -ForegroundColor Green
Write-Host "Testing completed!" -ForegroundColor Green

