package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/diasoft/gateway-service/internal/authctx"
	"github.com/diasoft/gateway-service/internal/model"
	"github.com/go-chi/chi/v5"
)

type DiplomaUseCase interface {
	Upload(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.BatchUploadResponse, error)
	GetBatch(ctx context.Context, batchID, vuzID string) (*model.Batch, error)
	DownloadBatch(ctx context.Context, batchID, vuzID string) ([]byte, error)
	Revoke(ctx context.Context, vuzID, hash string) error
}

type DiplomaHandler struct {
	diplomas  DiplomaUseCase
	validator Validator
}

func NewDiplomaHandler(diplomas DiplomaUseCase, validator Validator) *DiplomaHandler {
	return &DiplomaHandler{
		diplomas:  diplomas,
		validator: validator,
	}
}

func (h *DiplomaHandler) Upload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		slog.Info("received diploma upload request", "vuz_id", vuzID, "content_type", r.Header.Get("Content-Type"), "content_length", r.ContentLength)

		records, err := h.parseRecords(r)
		if err != nil {
			slog.Warn("failed to parse diploma upload request", "vuz_id", vuzID, "error", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		slog.Info("parsed diploma upload records", "vuz_id", vuzID, "records_count", len(records))

		for index := range records {
			if ok, err := h.validator.ValidateStruct(&records[index]); !ok {
				slog.Warn("validation failed for diploma upload record", "vuz_id", vuzID, "record_index", index, "error", err)
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		response, err := h.diplomas.Upload(r.Context(), vuzID, records)
		if err != nil {
			slog.Error("failed to upload diplomas", "vuz_id", vuzID, "records_count", len(records), "error", err)
			writeError(w, http.StatusInternalServerError, "failed to upload diplomas")
			return
		}
		slog.Info("created diploma batch", "vuz_id", vuzID, "batch_id", response.BatchID, "records_count", len(records))

		writeJSON(w, http.StatusAccepted, response)
	}
}

func (h *DiplomaHandler) BatchStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		batchID := chi.URLParam(r, "batch_id")
		if batchID == "" {
			writeError(w, http.StatusBadRequest, "missing batch id")
			return
		}

		batch, err := h.diplomas.GetBatch(r.Context(), batchID, vuzID)
		if err != nil {
			writeError(w, http.StatusNotFound, "batch not found")
			return
		}

		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		writeJSON(w, http.StatusOK, batch)
	}
}

func (h *DiplomaHandler) Download() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		batchID := chi.URLParam(r, "batch_id")
		if batchID == "" {
			writeError(w, http.StatusBadRequest, "missing batch id")
			return
		}

		fileBytes, err := h.diplomas.DownloadBatch(r.Context(), batchID, vuzID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to build excel file")
			return
		}

		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("X-Batch-ID", batchID)
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="batch_`+batchID+`.xlsx"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fileBytes)
	}
}

func (h *DiplomaHandler) Revoke() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vuzID, ok := authctx.UniversityIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		diplomaHash := chi.URLParam(r, "diploma_hash")
		if diplomaHash == "" {
			writeError(w, http.StatusBadRequest, "missing diploma hash")
			return
		}

		if err := h.diplomas.Revoke(r.Context(), vuzID, diplomaHash); err != nil {
			writeError(w, http.StatusNotFound, "diploma not found")
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"diploma_hash": diplomaHash,
			"status":       model.DiplomaStatusRevoked,
		})
	}
}

func (h *DiplomaHandler) parseRecords(r *http.Request) ([]model.DiplomaUploadRecord, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))

	switch mediaType {
	case "", "application/json":
		request := &model.DiplomaUploadRequest{}
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			return nil, errors.New("invalid json payload")
		}
		if len(request.Diplomas) == 0 {
			return nil, errors.New("diplomas payload is empty")
		}
		return request.Diplomas, nil
	case "multipart/form-data":
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, errors.New("failed to parse multipart form")
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, errors.New("csv file is required in form field 'file'")
		}
		defer file.Close()

		return parseCSV(file)
	case "text/csv", "application/csv", "application/vnd.ms-excel":
		return parseCSV(r.Body)
	default:
		if strings.Contains(mediaType, "csv") {
			return parseCSV(r.Body)
		}
		return nil, errors.New("unsupported content type")
	}
}

func parseCSV(reader io.Reader) ([]model.DiplomaUploadRecord, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, errors.New("failed to read csv")
	}
	if len(rows) < 2 {
		return nil, errors.New("csv must contain header and at least one row")
	}

	headerIndex := map[string]int{}
	for index, header := range rows[0] {
		headerIndex[normalizeCSVHeader(header)] = index
	}

	requiredHeaders := []string{"full_name", "diploma_number", "specialty", "degree", "faculty", "year"}
	for _, header := range requiredHeaders {
		if _, ok := headerIndex[header]; !ok {
			return nil, errors.New("csv header is missing required column: " + header)
		}
	}

	result := make([]model.DiplomaUploadRecord, 0, len(rows)-1)
	for _, row := range rows[1:] {
		for _, header := range requiredHeaders {
			if headerIndex[header] >= len(row) {
				return nil, errors.New("csv row has missing values")
			}
		}

		yearValue, err := strconv.Atoi(strings.TrimSpace(row[headerIndex["year"]]))
		if err != nil {
			return nil, errors.New("invalid year in csv")
		}

		result = append(result, model.DiplomaUploadRecord{
			FullName:      strings.TrimSpace(row[headerIndex["full_name"]]),
			DiplomaNumber: strings.TrimSpace(row[headerIndex["diploma_number"]]),
			Specialty:     strings.TrimSpace(row[headerIndex["specialty"]]),
			Degree:        strings.TrimSpace(row[headerIndex["degree"]]),
			Faculty:       strings.TrimSpace(row[headerIndex["faculty"]]),
			Year:          yearValue,
			RawCSVRow:     strings.Join(row, ","),
		})
	}

	return result, nil
}

func normalizeCSVHeader(header string) string {
	normalized := strings.TrimSpace(header)
	return strings.TrimPrefix(normalized, "\uFEFF")
}
