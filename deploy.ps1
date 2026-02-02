# PowerShell скрипт для развертывания в Kubernetes

Write-Host "==========================================" -ForegroundColor Green
Write-Host "IoT Stream Processor - Kubernetes Deployment" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green

# Проверка наличия kubectl
if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
    Write-Host "Error: kubectl not found. Please install kubectl." -ForegroundColor Red
    exit 1
}

# Проверка подключения к кластеру
try {
    kubectl cluster-info | Out-Null
} catch {
    Write-Host "Error: Cannot connect to Kubernetes cluster." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "1. Creating namespace..." -ForegroundColor Yellow
kubectl apply -f k8s/namespace.yaml

Write-Host ""
Write-Host "2. Creating ConfigMap..." -ForegroundColor Yellow
kubectl apply -f k8s/configmap.yaml

Write-Host ""
Write-Host "3. Deploying Redis..." -ForegroundColor Yellow
kubectl apply -f k8s/redis-deployment.yaml

Write-Host ""
Write-Host "4. Waiting for Redis to be ready..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=redis -n iot-stream-processor --timeout=120s

Write-Host ""
Write-Host "5. Building Docker image..." -ForegroundColor Yellow
# Для Minikube используем локальный реестр
if (Get-Command minikube -ErrorAction SilentlyContinue) {
    $minikubeStatus = minikube status 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Using Minikube Docker environment..." -ForegroundColor Cyan
        minikube docker-env | Invoke-Expression
        docker build -t iot-processor:latest .
        minikube docker-env -u | Invoke-Expression
    } else {
        Write-Host "Building Docker image locally..." -ForegroundColor Cyan
        docker build -t iot-processor:latest .
    }
} else {
    Write-Host "Building Docker image locally..." -ForegroundColor Cyan
    docker build -t iot-processor:latest .
    Write-Host "Note: For production, push image to registry:" -ForegroundColor Yellow
    Write-Host "  docker tag iot-processor:latest your-registry/iot-processor:latest"
    Write-Host "  docker push your-registry/iot-processor:latest"
}

Write-Host ""
Write-Host "6. Deploying application..." -ForegroundColor Yellow
kubectl apply -f k8s/app-deployment.yaml

Write-Host ""
Write-Host "7. Waiting for application pods to be ready..." -ForegroundColor Yellow
kubectl wait --for=condition=ready pod -l app=iot-processor -n iot-stream-processor --timeout=120s

Write-Host ""
Write-Host "8. Creating HPA..." -ForegroundColor Yellow
kubectl apply -f k8s/hpa.yaml

Write-Host ""
Write-Host "9. Creating Ingress (optional)..." -ForegroundColor Yellow
kubectl apply -f k8s/ingress.yaml

Write-Host ""
Write-Host "10. Installing Prometheus and Grafana..." -ForegroundColor Yellow
# Проверка наличия Helm
if (-not (Get-Command helm -ErrorAction SilentlyContinue)) {
    Write-Host "Warning: Helm not found. Skipping Prometheus/Grafana installation." -ForegroundColor Yellow
    Write-Host "Install Helm from https://helm.sh/docs/intro/install/" -ForegroundColor Yellow
} else {
    # Добавление репозитория
    Write-Host "Adding Prometheus Helm repository..." -ForegroundColor Cyan
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    
    # Проверка, не установлен ли уже Prometheus
    $prometheusInstalled = helm list -n monitoring -q 2>$null | Select-String -Pattern "prometheus"
    if (-not $prometheusInstalled) {
        Write-Host "Installing kube-prometheus-stack (this may take 3-5 minutes)..." -ForegroundColor Cyan
        Write-Host "Please wait, installing Prometheus, Grafana, and Alertmanager..." -ForegroundColor Yellow
        
        # Устанавливаем 
        $installResult = helm install prometheus prometheus-community/kube-prometheus-stack `
            --namespace monitoring `
            --create-namespace `
            --set prometheus.prometheusSpec.retention=30d `
            --set grafana.adminPassword=admin `
            --set grafana.service.type=NodePort `
            --set prometheus.service.type=NodePort `
            2>&1
        
        if ($LASTEXITCODE -eq 0) {
            Write-Host "Prometheus stack installation started successfully!" -ForegroundColor Green
            Write-Host "Waiting for pods to be ready (this may take a few minutes)..." -ForegroundColor Yellow
            
            # Ждем готовности подов в фоне с прогрессом
            $maxWait = 300  # 5 минут максимум
            $elapsed = 0
            $interval = 10  # проверяем каждые 10 секунд
            
            while ($elapsed -lt $maxWait) {
                Start-Sleep -Seconds $interval
                $elapsed += $interval
                
                $readyPods = kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>$null
                $prometheusReady = kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>$null
                
                if ($readyPods -match "True" -and $prometheusReady -match "True") {
                    Write-Host "Prometheus stack is ready!" -ForegroundColor Green
                    break
                }
                
                Write-Host "Still waiting... ($elapsed/$maxWait seconds)" -ForegroundColor Gray
            }
            
            if ($elapsed -ge $maxWait) {
                Write-Host "Warning: Timeout waiting for Prometheus stack. It may still be starting." -ForegroundColor Yellow
                Write-Host "Check status with: kubectl get pods -n monitoring" -ForegroundColor Yellow
            }
        } else {
            Write-Host "Error installing Prometheus stack:" -ForegroundColor Red
            Write-Host $installResult -ForegroundColor Red
        }
    } else {
        Write-Host "Prometheus stack already installed, skipping..." -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "11. Creating ServiceMonitor..." -ForegroundColor Yellow
kubectl apply -f k8s/servicemonitor.yaml

Write-Host ""
Write-Host "12. Creating Grafana Dashboard..." -ForegroundColor Yellow
kubectl apply -f k8s/grafana-dashboard.yaml

Write-Host ""
Write-Host "13. Creating Prometheus Alert Rules..." -ForegroundColor Yellow
kubectl apply -f k8s/prometheus-rules.yaml

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "Deployment completed!" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Check status:" -ForegroundColor Cyan
Write-Host "  kubectl get pods -n iot-stream-processor"
Write-Host "  kubectl get svc -n iot-stream-processor"
Write-Host "  kubectl get hpa -n iot-stream-processor"
Write-Host ""
Write-Host "Port forward to access service:" -ForegroundColor Cyan
Write-Host "  kubectl port-forward -n iot-stream-processor svc/iot-processor-service 8080:80"
Write-Host ""
Write-Host "Access Prometheus:" -ForegroundColor Cyan
Write-Host "  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090"
Write-Host "  Then open: http://localhost:9090"
Write-Host ""
Write-Host "Access Grafana:" -ForegroundColor Cyan
Write-Host "  kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80"
Write-Host "  Then open: http://localhost:3000"
Write-Host "  Login: admin / Password: admin"
Write-Host ""
Write-Host "Or get NodePort:" -ForegroundColor Cyan
Write-Host "  kubectl get svc -n monitoring prometheus-grafana"
Write-Host "  kubectl get svc -n monitoring prometheus-kube-prometheus-prometheus"
Write-Host ""
Write-Host "Test health endpoint:" -ForegroundColor Cyan
Write-Host "  curl http://localhost:8080/health"
Write-Host ""

