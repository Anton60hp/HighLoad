#!/bin/bash
# Скрипт для развертывания в Kubernetes

set -e

echo "=========================================="
echo "IoT Stream Processor - Kubernetes Deployment"
echo "=========================================="

# Проверка наличия kubectl
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found. Please install kubectl."
    exit 1
fi

# Проверка подключения к кластеру
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot connect to Kubernetes cluster."
    exit 1
fi

echo ""
echo "1. Creating namespace..."
kubectl apply -f k8s/namespace.yaml

echo ""
echo "2. Creating ConfigMap..."
kubectl apply -f k8s/configmap.yaml

echo ""
echo "3. Deploying Redis..."
kubectl apply -f k8s/redis-deployment.yaml

echo ""
echo "4. Waiting for Redis to be ready..."
kubectl wait --for=condition=ready pod -l app=redis -n iot-stream-processor --timeout=120s

echo ""
echo "5. Building Docker image..."
# Для Minikube используем локальный реестр
if command -v minikube &> /dev/null && minikube status &> /dev/null; then
    echo "Using Minikube Docker environment..."
    eval $(minikube docker-env)
    docker build -t iot-processor:latest .
    eval $(minikube docker-env -u)
else
    echo "Building Docker image locally..."
    docker build -t iot-processor:latest .
    echo "Note: For production, push image to registry:"
    echo "  docker tag iot-processor:latest your-registry/iot-processor:latest"
    echo "  docker push your-registry/iot-processor:latest"
fi

echo ""
echo "6. Deploying application..."
kubectl apply -f k8s/app-deployment.yaml

echo ""
echo "7. Waiting for application pods to be ready..."
kubectl wait --for=condition=ready pod -l app=iot-processor -n iot-stream-processor --timeout=120s

echo ""
echo "8. Creating HPA..."
kubectl apply -f k8s/hpa.yaml

echo ""
echo "9. Creating Ingress (optional)..."
kubectl apply -f k8s/ingress.yaml

echo ""
echo "10. Installing Prometheus and Grafana..."
# Проверка наличия Helm
if ! command -v helm &> /dev/null; then
    echo "Warning: Helm not found. Skipping Prometheus/Grafana installation."
    echo "Install Helm from https://helm.sh/docs/intro/install/"
else
    # Добавление репозитория
    echo "Adding Prometheus Helm repository..."
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo update
    
    # Проверка, не установлен ли уже Prometheus
    if ! helm list -n monitoring 2>/dev/null | grep -q prometheus; then
        echo "Installing kube-prometheus-stack (this may take 3-5 minutes)..."
        echo "Please wait, installing Prometheus, Grafana, and Alertmanager..."
        
        # Устанавливаем без --wait для более быстрого ответа
        if helm install prometheus prometheus-community/kube-prometheus-stack \
            --namespace monitoring \
            --create-namespace \
            --set prometheus.prometheusSpec.retention=30d \
            --set grafana.adminPassword=admin \
            --set grafana.service.type=NodePort \
            --set prometheus.service.type=NodePort; then
            
            echo "Prometheus stack installation started successfully!"
            echo "Waiting for pods to be ready (this may take a few minutes)..."
            
            # Ждем готовности подов с прогрессом
            max_wait=300  # 5 минут максимум
            elapsed=0
            interval=10  # проверяем каждые 10 секунд
            
            while [ $elapsed -lt $max_wait ]; do
                sleep $interval
                elapsed=$((elapsed + interval))
                
                grafana_ready=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
                prometheus_ready=$(kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
                
                if echo "$grafana_ready" | grep -q "True" && echo "$prometheus_ready" | grep -q "True"; then
                    echo "Prometheus stack is ready!"
                    break
                fi
                
                echo "Still waiting... ($elapsed/$max_wait seconds)"
            done
            
            if [ $elapsed -ge $max_wait ]; then
                echo "Warning: Timeout waiting for Prometheus stack. It may still be starting."
                echo "Check status with: kubectl get pods -n monitoring"
            fi
        else
            echo "Error installing Prometheus stack"
            exit 1
        fi
    else
        echo "Prometheus stack already installed, skipping..."
    fi
fi

echo ""
echo "11. Creating ServiceMonitor..."
kubectl apply -f k8s/servicemonitor.yaml

echo ""
echo "12. Creating Grafana Dashboard..."
kubectl apply -f k8s/grafana-dashboard.yaml

echo ""
echo "13. Creating Prometheus Alert Rules..."
kubectl apply -f k8s/prometheus-rules.yaml

echo ""
echo "=========================================="
echo "Deployment completed!"
echo "=========================================="
echo ""
echo "Check status:"
echo "  kubectl get pods -n iot-stream-processor"
echo "  kubectl get svc -n iot-stream-processor"
echo "  kubectl get hpa -n iot-stream-processor"
echo ""
echo "Port forward to access service:"
echo "  kubectl port-forward -n iot-stream-processor svc/iot-processor-service 8080:80"
echo ""
echo "Access Prometheus:"
echo "  kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090"
echo "  Then open: http://localhost:9090"
echo ""
echo "Access Grafana:"
echo "  kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80"
echo "  Then open: http://localhost:3000"
echo "  Login: admin / Password: admin"
echo ""
echo "Or get NodePort:"
echo "  kubectl get svc -n monitoring prometheus-grafana"
echo "  kubectl get svc -n monitoring prometheus-kube-prometheus-prometheus"
echo ""
echo "Test health endpoint:"
echo "  curl http://localhost:8080/health"
echo ""

