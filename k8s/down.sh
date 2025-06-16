#!/bin/bash

echo "Shutting down all services..."

# Delete all deployments
echo "Deleting deployments..."
kubectl delete deployment consumer
kubectl delete deployment aggregator
kubectl delete deployment performance-status
kubectl delete deployment redis
kubectl delete deployment rabbitmq

# Delete all services
echo "Deleting services..."
kubectl delete service consumer
kubectl delete service aggregator
kubectl delete service performance-status
kubectl delete service redis
kubectl delete service rabbitmq

# Wait for all pods to terminate
echo "Waiting for pods to terminate..."
kubectl wait --for=delete pod -l app=consumer --timeout=60s
kubectl wait --for=delete pod -l app=aggregator --timeout=60s
kubectl wait --for=delete pod -l app=performance-status --timeout=60s
kubectl wait --for=delete pod -l app=redis --timeout=60s
kubectl wait --for=delete pod -l app=rabbitmq --timeout=60s

# Verify all resources are gone
echo "Verifying cleanup..."
kubectl get pods
kubectl get services
kubectl get deployments

echo "All services have been shut down!" 