package postgres

import (
	"errors"
	"strings"

	"github.com/diasoft/gateway-service/internal/infrastructure/security"
)

type QRPayloadDecoder interface {
	Parse(payload string) (*security.ParsedQRPayload, error)
}

func parseQRPayload(decoder QRPayloadDecoder, payload string) (*security.ParsedQRPayload, error) {
	if decoder == nil {
		return nil, errors.New("qr payload decoder is not configured")
	}

	return decoder.Parse(payload)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
