package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

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
			MaxWait: 500 * time.Millisecond,
		}),
		handler: handler,
		logger:  logger,
	}
}

func (c *ResultConsumer) Start(ctx context.Context) {
	c.logger.Info("starting kafka result consumer")

	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("stopping kafka result consumer", "reason", ctx.Err())
				return
			}

			c.logger.Error("failed to fetch kafka message", "error", err)
			continue
		}

		c.logger.Info(
			"received kafka result message",
			"topic", message.Topic,
			"partition", message.Partition,
			"offset", message.Offset,
			"key", string(message.Key),
		)

		var result model.KafkaProcessingResult
		if err := json.Unmarshal(message.Value, &result); err != nil {
			c.logger.Error("failed to decode kafka result", "error", err)
			if commitErr := c.reader.CommitMessages(ctx, message); commitErr != nil {
				c.logger.Error("failed to commit malformed kafka message", "error", commitErr)
			} else {
				c.logger.Info(
					"committed malformed kafka message",
					"topic", message.Topic,
					"partition", message.Partition,
					"offset", message.Offset,
				)
			}
			continue
		}

		c.logger.Info(
			"decoded kafka result",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"vuz_id", result.VUZID,
			"status", result.Status,
			"has_qr_payload", result.QRPayload != nil,
			"has_error", result.Error != nil,
		)

		if err := c.handler.HandleProcessingResult(ctx, &result); err != nil {
			c.logger.Error("failed to apply kafka result", "batch_id", result.BatchID, "record_index", result.RecordIndex, "error", err)
			continue
		}

		c.logger.Info(
			"applied kafka result to database",
			"batch_id", result.BatchID,
			"record_index", result.RecordIndex,
			"status", result.Status,
		)

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			c.logger.Error("failed to commit kafka message", "error", err)
		} else {
			c.logger.Info(
				"committed kafka result message",
				"topic", message.Topic,
				"partition", message.Partition,
				"offset", message.Offset,
				"batch_id", result.BatchID,
				"record_index", result.RecordIndex,
			)
		}
	}
}

func (c *ResultConsumer) Close() error {
	return c.reader.Close()
}
