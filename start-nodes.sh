#!/bin/bash

# Number of nodes to start
NUM_NODES=${1:-3}

# Function to check if a command was successful
check_error() {
    if [ $? -ne 0 ]; then
        echo "Error: $1"
        exit 1
    fi
}

# Function to generate docker-compose.yml with dynamic nodes
generate_compose_file() {
    local num_nodes=$1
    local compose_file="docker-compose.yml"
    local temp_file="docker-compose.temp.yml"

    # Create temporary file with base services
    cat > "$temp_file" << EOF
services:
  aggregator:
    build: ./aggregator
    container_name: aggregator
    environment:
      - PORT=8080
      - NUM_NODES=$NUM_NODES
    ports:
      - "8001:8080"
    networks:
      - app-network
    depends_on:
      - rabbitmq
EOF

    # Add node dependencies to aggregator
    for i in $(seq 1 $num_nodes); do
        echo "      - node$i" >> "$temp_file"
    done

    # Add node services
    for i in $(seq 1 $num_nodes); do
        cat >> "$temp_file" << EOF

  node$i:
    build: ./performance-status
    container_name: node$i
    environment:
      - PORT=8080
    networks:
      - app-network
EOF
    done

    # Add remaining services
    cat >> "$temp_file" << EOF

  redis:
    image: redis:latest
    container_name: redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    networks:
      - app-network

  rabbitmq:
    image: rabbitmq:3-management
    container_name: rabbitmq
    ports:
      - "5672:5672"
      - "15672:15672"
    networks:
      - app-network

  consumer:
    build: ./consumer
    container_name: consumer
    environment:
      - PORT=8080
    networks:
      - app-network
    depends_on:
      - redis
      - rabbitmq
      - aggregator

volumes:
  redis_data:

networks:
  app-network:
    driver: bridge
EOF

    # Replace original file with new one
    mv "$temp_file" "$compose_file"
}

# Generate docker-compose.yml with desired number of nodes
echo "Generating docker-compose.yml with $NUM_NODES nodes..."
generate_compose_file $NUM_NODES
check_error "Failed to generate docker-compose.yml"

# Clean up any existing containers
echo "Cleaning up existing containers..."
docker-compose down
check_error "Failed to clean up existing containers"

# Start all services
echo "Starting all services..."
docker-compose up -d --build
check_error "Failed to start services"

# echo "Waiting for services to be ready..."

# Wait for nodes to be ready
# for i in $(seq 1 $NUM_NODES); do
#     while true; do
#         if curl -s "http://node$i:8080/status" > /dev/null; then
#             echo "Node$i is ready"
#             break
#         fi
#         echo "Waiting for node$i to be ready..."
#         sleep 2
#     done
# done

echo "All services are up and running!"

# Function to handle cleanup on script exit
cleanup() {
    echo "Cleaning up..."
    docker-compose down
}

# Register the cleanup function to run on script exit
# trap cleanup EXIT 