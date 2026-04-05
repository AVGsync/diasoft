package analytics

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

type VerificationEvent struct {
	EventID      string    `json:"event_id"`
	CreatedAt    time.Time `json:"created_at"`
	Source       string    `json:"source_service"`
	Endpoint     string    `json:"endpoint"`
	RequestID    string    `json:"request_id,omitempty"`
	VUZID        string    `json:"vuz_id,omitempty"`
	VUZCode      string    `json:"vuz_code,omitempty"`
	DiplomaHash  string    `json:"diploma_hash,omitempty"`
	Status       string    `json:"status"`
	Valid        bool      `json:"is_valid"`
	Country      string    `json:"country,omitempty"`
	City         string    `json:"city,omitempty"`
	ClientIPHash string    `json:"client_ip_hash,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`

	ClientIP string `json:"-"`
}

type Tracker interface {
	Track(VerificationEvent)
	Close() error
}

type GeoResolver interface {
	Resolve(ip string) (country, city string)
	Close() error
}

type noopGeoResolver struct{}

func (noopGeoResolver) Resolve(string) (string, string) { return "", "" }
func (noopGeoResolver) Close() error                    { return nil }

type KafkaConfig struct {
	Brokers      []string
	Topic        string
	ClientID     string
	WriteTimeout time.Duration
	QueueSize    int
}

type KafkaTracker struct {
	logger      *slog.Logger
	writer      *kafkago.Writer
	queue       chan VerificationEvent
	geoResolver GeoResolver
	wg          sync.WaitGroup
	closeOnce   sync.Once
}

func NewKafkaTracker(logger *slog.Logger, cfg KafkaConfig, geoResolver GeoResolver) *KafkaTracker {
	if logger == nil {
		logger = slog.Default()
	}
	if geoResolver == nil {
		geoResolver = noopGeoResolver{}
	}

	tracker := &KafkaTracker{
		logger: logger,
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(cfg.Brokers...),
			Topic:        cfg.Topic,
			Balancer:     &kafkago.LeastBytes{},
			WriteTimeout: cfg.WriteTimeout,
			RequiredAcks: kafkago.RequireOne,
			Async:        false,
			Transport: &kafkago.Transport{
				ClientID: cfg.ClientID,
			},
		},
		queue:       make(chan VerificationEvent, cfg.QueueSize),
		geoResolver: geoResolver,
	}

	tracker.wg.Add(1)
	go tracker.run()

	return tracker
}

func (t *KafkaTracker) Track(event VerificationEvent) {
	if event.EventID == "" {
		event.EventID = randomEventID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	select {
	case t.queue <- event:
	default:
		t.logger.Warn("analytics event dropped because queue is full", "endpoint", event.Endpoint, "status", event.Status, "vuz_id", event.VUZID, "vuz_code", event.VUZCode)
	}
}

func (t *KafkaTracker) Close() error {
	var closeErr error
	t.closeOnce.Do(func() {
		close(t.queue)
		t.wg.Wait()
		if err := t.writer.Close(); err != nil {
			closeErr = err
		}
		if err := t.geoResolver.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

func (t *KafkaTracker) run() {
	defer t.wg.Done()

	for event := range t.queue {
		enriched := t.enrich(event)

		payload, err := json.Marshal(enriched)
		if err != nil {
			t.logger.Error("marshal analytics event", "error", err)
			continue
		}

		writeCtx, cancel := context.WithTimeout(context.Background(), t.writer.WriteTimeout)
		err = t.writer.WriteMessages(writeCtx, kafkago.Message{
			Key:   []byte(eventPartitionKey(enriched)),
			Value: payload,
			Time:  enriched.CreatedAt,
		})
		cancel()
		if err != nil {
			t.logger.Error("publish analytics event", "error", err, "endpoint", enriched.Endpoint, "status", enriched.Status)
		}
	}
}

func (t *KafkaTracker) enrich(event VerificationEvent) VerificationEvent {
	event.ClientIP = strings.TrimSpace(event.ClientIP)
	if event.ClientIP != "" {
		event.ClientIPHash = hashIP(event.ClientIP)
		if country, city := t.geoResolver.Resolve(event.ClientIP); country != "" || city != "" {
			event.Country = country
			event.City = city
		}
	}
	event.ClientIP = ""
	event.VUZCode = strings.TrimSpace(event.VUZCode)
	event.VUZID = strings.TrimSpace(event.VUZID)
	event.DiplomaHash = strings.TrimSpace(event.DiplomaHash)
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.UserAgent = strings.TrimSpace(event.UserAgent)
	return event
}

func eventPartitionKey(event VerificationEvent) string {
	switch {
	case event.VUZID != "":
		return event.VUZID
	case event.VUZCode != "":
		return event.VUZCode
	case event.RequestID != "":
		return event.RequestID
	default:
		return event.EventID
	}
}

func hashIP(ip string) string {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return ""
	}
	if addr, err := netip.ParseAddr(trimmed); err == nil {
		trimmed = addr.String()
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

func randomEventID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}
