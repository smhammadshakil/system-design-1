#!/bin/bash

# Build Docker images
echo "Building Docker images..."
docker build -t performance-status:latest ./performance-status
docker build -t aggregator:latest ./aggregator
docker build -t consumer:latest ./consumer
docker build -t monitoring:latest ./monitoring

# Load images into minikube
echo "Loading images into minikube..."
minikube image load performance-status:latest
minikube image load aggregator:latest
minikube image load consumer:latest
minikube image load monitoring:latest

# Apply Kubernetes manifests
echo "Deploying to Kubernetes..."
kubectl apply -f k8s/redis-deployment.yaml
kubectl apply -f k8s/rabbitmq-deployment.yaml
kubectl apply -f k8s/postgres-deployment.yaml
kubectl apply -f k8s/pgadmin-deployment.yaml
kubectl apply -f k8s/nodes-deployment.yaml
kubectl apply -f k8s/aggregator-deployment.yaml
kubectl apply -f k8s/consumer-deployment.yaml
kubectl apply -f k8s/monitoring-deployment.yaml

# Wait for deployments to be ready
echo "Waiting for deployments to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/redis
kubectl wait --for=condition=available --timeout=300s deployment/rabbitmq
kubectl wait --for=condition=available --timeout=300s deployment/postgres
kubectl wait --for=condition=available --timeout=300s deployment/pgadmin
kubectl wait --for=condition=available --timeout=300s deployment/performance-status
kubectl wait --for=condition=available --timeout=300s deployment/aggregator
kubectl wait --for=condition=available --timeout=300s deployment/consumer
kubectl wait --for=condition=available --timeout=300s deployment/monitoring

echo "Deployment completed!"
echo "You can access the services using:"
echo "RabbitMQ Management: http://localhost:15672 (guest/guest)"
echo "pgAdmin: http://localhost:5050 (admin@admin.com/admin)"
echo "To port-forward services, use:"
echo "kubectl port-forward service/rabbitmq 15672:15672 &"
echo "kubectl port-forward service/pgadmin 5050:80 &"
echo "kubectl port-forward service/aggregator 8001:8080 &"
echo "kubectl port-forward service/monitoring 8080:8080 &" 