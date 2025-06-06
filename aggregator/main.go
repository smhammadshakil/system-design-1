package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
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

func getOwnSubnet() (string, error) {
	// Get container's IP address
	cmd := exec.Command("hostname", "-i")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting IP address: %v", err)
	}

	// Get the first IP address (in case there are multiple)
	ips := strings.Fields(string(output))
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP address found")
	}

	// Extract the first three octets to form the subnet
	ipParts := strings.Split(ips[0], ".")
	if len(ipParts) != 4 {
		return "", fmt.Errorf("invalid IP address format")
	}

	// Create subnet in CIDR notation (e.g., 172.20.0.0/16)
	subnet := fmt.Sprintf("%s.%s.0.0/24", ipParts[0], ipParts[1])
	return subnet, nil
}

func discoverNodes() []string {
	// Get the subnet based on container's own IP
	subnet, err := getOwnSubnet()
	if err != nil {
		fmt.Printf("Error getting subnet: %v\n", err)
		return nil
	}

	// Use nmap to find nodes on the network
	fmt.Printf("- - Finding Nodes on subnet %s - -\n", subnet)
	cmd := exec.Command("nmap", "-sn", "-n", subnet)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running nmap: %v\n", err)
		return nil
	}
	fmt.Printf("- - Nodes Found - -\n")
	var endpoints []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Nmap scan report for") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				endpoints = append(endpoints, fmt.Sprintf("http://%s:8080/status", fields[4]))
			}
		}
	}

	if len(endpoints) == 0 {
		fmt.Println("No nodes found on the network")
		return nil
	}

	fmt.Printf("Discovered %d nodes: %v\n", len(endpoints), endpoints)
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

func fetchAllEndpoints(endpoints []string) []Response {
	if len(endpoints) == 0 {
		fmt.Println("No nodes discovered on the network")
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

func createQueue() (*amqp.Connection, *amqp.Channel) {
	var conn *amqp.Connection
	var ch *amqp.Channel
	var err error

	// Retry connection with backoff
	maxRetries := 30
	retryInterval := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		fmt.Printf("Attempting to connect to RabbitMQ (attempt %d/%d)...\n", i+1, maxRetries)
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			fmt.Println("Successfully connected to RabbitMQ")
			break
		}
		fmt.Printf("Failed to connect to RabbitMQ: %v. Retrying in %v...\n", err, retryInterval)
		time.Sleep(retryInterval)
	}

	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ after all retries")
		panic(err)
	}

	// Create channel
	ch, err = conn.Channel()
	if err != nil {
		fmt.Println("Error opening channel", err)
		panic(err)
	}

	// Declare queue
	_, err = ch.QueueDeclare(
		"performance_status", // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		fmt.Println("Error declaring queue", err)
		panic(err)
	}
	fmt.Println("Queue 'performance_status' declared successfully")
	return conn, ch
}

func publishToRabbitMQ(ch *amqp.Channel, messages []Response) {
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

	// Discover nodes
	endpoints := discoverNodes()
	conn, ch := createQueue()
	defer conn.Close()
	defer ch.Close()
	r.GET("/aggregate", func(c *gin.Context) {
		responses := fetchAllEndpoints(endpoints)
		if len(responses) == 0 {
			c.JSON(500, gin.H{"error": "No valid responses received"})
			return
		}
		publishToRabbitMQ(ch, responses)
		c.JSON(200, responses)
	})

	// Create a channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a done channel to stop the ticker
	done := make(chan bool)

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				responses := fetchAllEndpoints(endpoints)
				if len(responses) > 0 {
					publishToRabbitMQ(ch, responses)
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
