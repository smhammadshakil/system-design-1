#!/bin/bash

# Function to update a specific service
update_service() {
    local service=$1
    local image=$2

    echo "Updating $service..."
    
    # Build new Docker image with timestamp to ensure new image
    TIMESTAMP=$(date +%s)
    NEW_IMAGE="${image}:${TIMESTAMP}"
    
    echo "Building new Docker image for $service..."
    docker build -t $NEW_IMAGE ./$service
    docker tag $NEW_IMAGE $image:latest

    # Load image into minikube
    echo "Loading new image into minikube..."
    minikube image load $NEW_IMAGE
    minikube image load $image:latest

    # Delete existing pods to force recreation
    echo "Deleting existing pods..."
    kubectl delete pods -l app=$service

    # Update deployment with new image
    echo "Updating deployment with new image..."
    kubectl set image deployment/$service $service=$NEW_IMAGE

    # Wait for rollout to complete
    echo "Waiting for $service to be ready..."
    kubectl rollout status deployment/$service

    # Verify the image update
    echo "Verifying image update..."
    kubectl get pods -l app=$service -o jsonpath='{.items[*].spec.containers[*].image}'
    echo

    # Get pod names
    echo "Getting pod names..."
    PODS=$(kubectl get pods -l app=$service -o jsonpath='{.items[*].metadata.name}')
    echo "Pods: $PODS"

    echo "$service update completed!"
}

# Check if service name is provided
if [ -z "$1" ]; then
    echo "Usage: ./update.sh <service-name>"
    echo "Available services: consumer, aggregator, performance-status, monitoring"
    exit 1
fi

# Update the specified service
case $1 in
    "consumer")
        update_service "consumer" "consumer"
        ;;
    "aggregator")
        update_service "aggregator" "aggregator"
        ;;
    "performance-status")
        update_service "performance-status" "performance-status"
        ;;
    "monitoring")
        update_service "monitoring" "monitoring"
        ;;
    *)
        echo "Invalid service name. Available services: consumer, aggregator, performance-status, monitoring"
        exit 1
        ;;
esac 