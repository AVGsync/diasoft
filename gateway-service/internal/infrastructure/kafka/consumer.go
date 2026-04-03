package kafka

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/diasoft/gateway-service/internal/model"
	kafkago "github.com/segmentio/kafka-go"
)

type ResultHandler interface {
	HandleProcessingResult(ctx context.Context, result *model.KafkaProcessingResult) error
}

type ResultConsumer struct {
	reader  *kafkago.Reader
	handler ResultHandler
	logger  *slog.Logger
}

func NewResultConsumer(config *Config, handler ResultHandler, logger *slog.Logger) *ResultConsumer {
	return &ResultConsumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: config.Brokers,
			GroupID: config.ConsumerGroup,
			Topic:   config.ProcessingResultsTopic,
			MaxWait: 500,
		}),
		handler: handler,
		logger:  logger,
	}
}

func (c *ResultConsumer) Start(ctx context.Context) {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			c.logger.Error("failed to fetch kafka message", "error", err)
			continue
		}

		var result model.KafkaProcessingResult
		if err := json.Unmarshal(message.Value, &result); err != nil {
			c.logger.Error("failed to decode kafka result", "error", err)
			if commitErr := c.reader.CommitMessages(ctx, message); commitErr != nil {
				c.logger.Error("failed to commit malformed kafka message", "error", commitErr)
			}
			continue
		}

		if err := c.handler.HandleProcessingResult(ctx, &result); err != nil {
			c.logger.Error("failed to apply kafka result", "batch_id", result.BatchID, "record_index", result.RecordIndex, "error", err)
			continue
		}

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			c.logger.Error("failed to commit kafka message", "error", err)
		}
	}
}

func (c *ResultConsumer) Close() error {
	return c.reader.Close()
}
