package model

type KafkaRawTask struct {
	BatchID      string         `json:"batch_id"`
	VUZID        string         `json:"vuz_id"`
	RecordIndex  int            `json:"record_index"`
	TotalInBatch int            `json:"total_in_batch"`
	Student      DiplomaStudent `json:"student"`
	CreatedAt    string         `json:"created_at"`
}

type DiplomaStudent struct {
	FullName      string `json:"full_name"`
	DiplomaNumber string `json:"diploma_number"`
	Specialty     string `json:"specialty"`
	Degree        string `json:"degree"`
	Year          int    `json:"year"`
	Faculty       string `json:"faculty"`
	RawCSVRow     string `json:"raw_csv_row,omitempty"`
}

type KafkaProcessingResult struct {
	BatchID          string  `json:"batch_id"`
	VUZID            string  `json:"vuz_id"`
	RecordIndex      int     `json:"record_index"`
	DiplomaHash      string  `json:"diploma_hash"`
	Signature        *string `json:"signature"`
	EncryptedPayload *string `json:"encrypted_payload"`
	QRPayload        *string `json:"qr_payload"`
	Status           string  `json:"status"`
	Error            *string `json:"error"`
	ProcessedAt      string  `json:"processed_at"`
}
