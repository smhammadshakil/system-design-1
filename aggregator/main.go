package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

type Response struct {
	Endpoint string `json:"endpoint"`
	Value    int    `json:"value"`
}

func discoverNodes() []string {
	// Get number of nodes from environment variable, default to 3
	numNodes := 3
	if numStr := os.Getenv("NUM_NODES"); numStr != "" {
		if n, err := strconv.Atoi(numStr); err == nil {
			numNodes = n
		}
	}

	var endpoints []string
	for i := 1; i <= numNodes; i++ {
		endpoints = append(endpoints, fmt.Sprintf("http://node%d:8080/status", i))
	}
	return endpoints
}

func fetchEndpoint(url string, wg *sync.WaitGroup, results chan<- Response) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response from %s: %v\n", url, err)
		return
	}

	num, err := strconv.Atoi(string(body))
	if err != nil {
		fmt.Printf("Error converting response from %s: %v\n", url, err)
		return
	}

	results <- Response{Endpoint: url, Value: num}
}

func fetchAllEndpoints() []Response {
	// Discover nodes
	endpoints := discoverNodes()
	if len(endpoints) == 0 {
		log.Fatal("No nodes discovered on the network")
	}

	var wg sync.WaitGroup
	results := make(chan Response, len(endpoints))

	for _, endpoint := range endpoints {
		wg.Add(1)
		go fetchEndpoint(endpoint, &wg, results)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var responses []Response
	for result := range results {
		fmt.Printf("Endpoint: %s, Value: %d\n", result.Endpoint, result.Value)
		responses = append(responses, result)
	}

	return responses
}

func publishToRabbitMQ(messages []Response) {
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		fmt.Println("Error connecting to RabbitMQ", err)
		panic(err)
	}
	defer conn.Close()
	fmt.Println("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Error opening channel", err)
		panic(err)
	}
	defer ch.Close()

	ch.QueueDeclare(
		"performance_status", // name
		false,                // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	fmt.Println("Queue declared")

	jsonData, err := json.Marshal(messages)
	if err != nil {
		fmt.Println("Error marshaling messages", err)
		panic(err)
	}

	ch.Publish(
		"",
		"performance_status",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        jsonData,
		},
	)
	fmt.Println("Message published")
}

func main() {
	server_port := os.Getenv("PORT")
	r := gin.Default()

	r.GET("/aggregate", func(c *gin.Context) {
		responses := fetchAllEndpoints()
		if len(responses) == 0 {
			c.JSON(500, gin.H{"error": "No valid responses received"})
			return
		}
		publishToRabbitMQ(responses)
		c.JSON(200, responses)
	})

	// Create a channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a done channel to stop the ticker
	done := make(chan bool)

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				responses := fetchAllEndpoints()
				if len(responses) > 0 {
					publishToRabbitMQ(responses)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	// Start the server in a goroutine
	go func() {
		if err := r.Run(":" + server_port); err != nil {
			fmt.Printf("Error starting server: %v\n", err)
			done <- true
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("Shutting down gracefully...")
	done <- true
	time.Sleep(1 * time.Second) // Give time for cleanup
}
