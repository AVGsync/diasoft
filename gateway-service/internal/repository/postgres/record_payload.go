package postgres

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/diasoft/gateway-service/internal/model"
)

type RecordPayloadCipher interface {
	Seal(plaintext []byte) (string, error)
	Open(value string) ([]byte, error)
}

func encodeBatchRecordPayload(cipher RecordPayloadCipher, record model.DiplomaUploadRecord) (string, error) {
	if cipher == nil {
		return "", errors.New("record payload cipher is not configured")
	}

	plaintext, err := json.Marshal(model.DiplomaUploadRecord{
		FullName:      record.FullName,
		DiplomaNumber: record.DiplomaNumber,
		Specialty:     record.Specialty,
		Degree:        record.Degree,
		Faculty:       record.Faculty,
		Year:          record.Year,
	})
	if err != nil {
		return "", err
	}

	return cipher.Seal(plaintext)
}

func decodeBatchRecordPayload(cipher RecordPayloadCipher, value string) (*model.DiplomaUploadRecord, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	if cipher == nil {
		return nil, errors.New("record payload cipher is not configured")
	}

	plaintext, err := cipher.Open(value)
	if err != nil {
		return nil, err
	}

	item := &model.DiplomaUploadRecord{}
	if err := json.Unmarshal(plaintext, item); err != nil {
		return nil, err
	}

	return item, nil
}
