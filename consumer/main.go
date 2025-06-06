package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/streadway/amqp"
)

type Response struct {
	Endpoint string `json:"endpoint"`
	Value    int    `json:"value"`
}

type Consumer struct {
	redisClient *redis.Client
	rabbitConn  *amqp.Connection
	channel     *amqp.Channel
	queue       amqp.Queue
	done        chan bool
}

func NewConsumer() (*Consumer, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})

	// Connect to RabbitMQ
	var conn *amqp.Connection
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

	ch, err := conn.Channel()
	if err != nil {
		fmt.Println("Error opening channel", err)
		panic(err)
	}

	// Declare queue
	q, err := ch.QueueDeclare(
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

	return &Consumer{
		redisClient: rdb,
		rabbitConn:  conn,
		channel:     ch,
		queue:       q,
		done:        make(chan bool),
	}, nil
}

func (c *Consumer) Close() {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.rabbitConn != nil {
		c.rabbitConn.Close()
	}
	if c.redisClient != nil {
		c.redisClient.Close()
	}
}

func (c *Consumer) storeInRedis(ctx context.Context, responses []Response) {
	timestamp := time.Now().Unix()
	for _, resp := range responses {
		key := fmt.Sprintf("%s:%d", resp.Endpoint, timestamp)
		value := fmt.Sprintf("%d", resp.Value)
		fmt.Printf("- - Data key: %v\n", resp.Endpoint)
		fmt.Printf("- - Data val: %v\n", resp.Value)

		fmt.Println("Sleeping for 15 seconds...")
		time.Sleep(15 * time.Second)
		if err := c.redisClient.Set(ctx, key, value, 24*time.Hour).Err(); err != nil {
			fmt.Printf("Error storing in Redis: %v\n", err)
		} else {
			fmt.Printf("Stored in Redis - Key: %s, Value: %s\n", key, value)
		}
	}
}

func (c *Consumer) Start() error {
	// Ensure channel is open
	if c.channel == nil {
		ch, err := c.rabbitConn.Channel()
		if err != nil {
			return fmt.Errorf("failed to open channel: %v", err)
		}
		c.channel = ch
	}

	// Set QoS to prefetch 1 message at a time
	err := c.channel.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %v", err)
	}
	fmt.Println("QoS settings applied: prefetch count = 1")

	msgs, err := c.channel.Consume(
		c.queue.Name, // queue
		"",           // consumer
		false,        // auto-ack (changed to false for manual ack)
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %v", err)
	}

	// Start connection monitoring
	go c.monitorConnection()

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					fmt.Println("Message channel closed, attempting to reconnect...")
					// Attempt to reconnect
					if err := c.reconnect(); err != nil {
						fmt.Printf("Failed to reconnect: %v\n", err)
						return
					}
					// Restart the consumer
					if err := c.Start(); err != nil {
						fmt.Printf("Failed to restart consumer: %v\n", err)
						return
					}
					return
				}

				var responses []Response
				if err := json.Unmarshal(msg.Body, &responses); err != nil {
					fmt.Printf("Error unmarshaling message: %v\n", err)
					msg.Ack(false) // Acknowledge the message even if it's invalid
					continue
				}

				fmt.Printf("Processing message with %d responses...\n", len(responses))
				c.storeInRedis(context.Background(), responses)
				fmt.Println("Message processing completed, acknowledging...")
				msg.Ack(false) // Manually acknowledge the message after processing

			case <-c.done:
				return
			}
		}
	}()

	return nil
}

func (c *Consumer) monitorConnection() {
	notifyClose := c.rabbitConn.NotifyClose(make(chan *amqp.Error))
	for {
		select {
		case err := <-notifyClose:
			if err != nil {
				fmt.Printf("Connection closed: %v\n", err)
				// Attempt to reconnect
				if err := c.reconnect(); err != nil {
					fmt.Printf("Failed to reconnect: %v\n", err)
					return
				}
				// Restart the consumer
				if err := c.Start(); err != nil {
					fmt.Printf("Failed to restart consumer: %v\n", err)
					return
				}
			}
		case <-c.done:
			return
		}
	}
}

func (c *Consumer) reconnect() error {
	// Close existing channel if it exists
	if c.channel != nil {
		c.channel.Close()
	}

	// Close existing connection if it exists
	if c.rabbitConn != nil {
		c.rabbitConn.Close()
	}

	// Retry connection with backoff
	maxRetries := 30
	retryInterval := 2 * time.Second

	var conn *amqp.Connection
	var err error

	for i := 0; i < maxRetries; i++ {
		fmt.Printf("Attempting to reconnect to RabbitMQ (attempt %d/%d)...\n", i+1, maxRetries)
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			fmt.Println("Successfully reconnected to RabbitMQ")
			break
		}
		fmt.Printf("Failed to reconnect to RabbitMQ: %v. Retrying in %v...\n", err, retryInterval)
		time.Sleep(retryInterval)
	}

	if err != nil {
		return fmt.Errorf("failed to reconnect after all retries: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %v", err)
	}

	// Update consumer's connection and channel
	c.rabbitConn = conn
	c.channel = ch

	// Re-declare queue
	q, err := ch.QueueDeclare(
		"performance_status", // name
		true,                 // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %v", err)
	}
	c.queue = q

	return nil
}

func (c *Consumer) Stop() {
	c.done <- true
}

func main() {
	fmt.Println("Waiting 10 seconds before starting the consumer...")
	time.Sleep(10 * time.Second)
	fmt.Println("Starting consumer...")

	consumer, err := NewConsumer()
	if err != nil {
		fmt.Printf("Error initializing consumer: %v\n", err)
		os.Exit(1)
	}
	defer consumer.Close()
	if err := consumer.Start(); err != nil {
		fmt.Printf("Error starting consumer: %v\n", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Consumer started. Press Ctrl+C to exit.")
	<-sigChan
	fmt.Println("Shutting down gracefully...")
	consumer.Stop()
	time.Sleep(1 * time.Second)
}
