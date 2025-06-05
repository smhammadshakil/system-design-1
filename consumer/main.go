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

	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}

	q, err := ch.QueueDeclare(
		"performance_status", // name
		false,                // durable
		false,                // delete when unused
		false,                // exclusive
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare a queue: %v", err)
	}

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

		if err := c.redisClient.Set(ctx, key, value, 24*time.Hour).Err(); err != nil {
			fmt.Printf("Error storing in Redis: %v\n", err)
		} else {
			fmt.Printf("Stored in Redis - Key: %s, Value: %s\n", key, value)
		}
	}
}

func (c *Consumer) Start() error {
	msgs, err := c.channel.Consume(
		c.queue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %v", err)
	}

	go func() {
		for {
			select {
			case msg := <-msgs:
				var responses []Response
				if err := json.Unmarshal(msg.Body, &responses); err != nil {
					fmt.Printf("Error unmarshaling message: %v\n", err)
					continue
				}
				c.storeInRedis(context.Background(), responses)

			case <-c.done:
				return
			}
		}
	}()

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
