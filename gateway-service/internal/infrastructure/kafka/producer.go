package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
	kafkago "github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafkago.Writer
	topic  string
}

func NewProducer(config *Config) *Producer {
	return &Producer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(config.Brokers...),
			Topic:        config.RawTasksTopic,
			Balancer:     &kafkago.LeastBytes{},
			RequiredAcks: kafkago.RequireOne,
			WriteTimeout: 10 * time.Second,
			ReadTimeout:  10 * time.Second,
			MaxAttempts:  3,
		},
		topic: config.RawTasksTopic,
	}
}

func (p *Producer) PublishRawTasks(ctx context.Context, tasks []*model.KafkaRawTask) error {
	writeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	messages := make([]kafkago.Message, 0, len(tasks))
	for _, task := range tasks {
		payload, err := json.Marshal(task)
		if err != nil {
			return err
		}
		messages = append(messages, kafkago.Message{
			Key:   []byte(task.BatchID),
			Value: payload,
		})
	}

	return p.writer.WriteMessages(writeCtx, messages...)
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
