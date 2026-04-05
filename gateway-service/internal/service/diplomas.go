package service

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/diasoft/gateway-service/internal/model"
)

type RawTaskProducer interface {
	PublishRawTasks(ctx context.Context, tasks []*model.KafkaRawTask) error
}

type ExcelGenerator interface {
	BuildBatch(rows []*model.BatchDownloadRow) ([]byte, error)
}

type SigningKeyStatusReader interface {
	FindByVUZID(ctx context.Context, vuzID string) (*model.UniversitySigningKey, error)
}

type DiplomaRepository interface {
	CreateBatchWithRecords(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.Batch, error)
	FailBatch(ctx context.Context, batchID string) error
	GetBatch(ctx context.Context, batchID, vuzID string) (*model.Batch, error)
	GetBatchDownloadRows(ctx context.Context, batchID, vuzID string) ([]*model.BatchDownloadRow, error)
	MarkDiplomaRevoked(ctx context.Context, vuzID, hash string) error
	ApplyProcessingResult(ctx context.Context, result *model.KafkaProcessingResult) error
}

type DiplomaService struct {
	repo        DiplomaRepository
	signingKeys SigningKeyStatusReader
	producer    RawTaskProducer
	excelFile   ExcelGenerator
}

func NewDiplomaService(repo DiplomaRepository, signingKeys SigningKeyStatusReader, producer RawTaskProducer, excelFile ExcelGenerator) *DiplomaService {
	return &DiplomaService{
		repo:        repo,
		signingKeys: signingKeys,
		producer:    producer,
		excelFile:   excelFile,
	}
}

func (s *DiplomaService) Upload(ctx context.Context, vuzID string, records []model.DiplomaUploadRecord) (*model.BatchUploadResponse, error) {
	if _, err := s.signingKeys.FindByVUZID(ctx, vuzID); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSigningKeyNotFound
		}
		return nil, err
	}

	batch, err := s.repo.CreateBatchWithRecords(ctx, vuzID, records)
	if err != nil {
		return nil, err
	}

	tasks := make([]*model.KafkaRawTask, 0, len(records))
	now := time.Now().UTC().Format(time.RFC3339)

	for index, record := range records {
		tasks = append(tasks, &model.KafkaRawTask{
			BatchID:      batch.ID,
			VUZID:        vuzID,
			RecordIndex:  index,
			TotalInBatch: len(records),
			CreatedAt:    now,
			Student: model.DiplomaStudent{
				FullName:      record.FullName,
				DiplomaNumber: record.DiplomaNumber,
				Specialty:     record.Specialty,
				Degree:        record.Degree,
				Year:          record.Year,
				Faculty:       record.Faculty,
				RawCSVRow:     record.RawCSVRow,
			},
		})
	}

	if err := s.producer.PublishRawTasks(ctx, tasks); err != nil {
		_ = s.repo.FailBatch(ctx, batch.ID)
		return nil, err
	}

	return &model.BatchUploadResponse{
		BatchID: batch.ID,
		Status:  batch.Status,
	}, nil
}

func (s *DiplomaService) GetBatch(ctx context.Context, batchID, vuzID string) (*model.Batch, error) {
	return s.repo.GetBatch(ctx, batchID, vuzID)
}

func (s *DiplomaService) DownloadBatch(ctx context.Context, batchID, vuzID string) ([]byte, error) {
	rows, err := s.repo.GetBatchDownloadRows(ctx, batchID, vuzID)
	if err != nil {
		return nil, err
	}
	slog.Info("building batch excel", "batch_id", batchID, "vuz_id", vuzID, "rows", len(rows))
	return s.excelFile.BuildBatch(rows)
}

func (s *DiplomaService) Revoke(ctx context.Context, vuzID, hash string) error {
	return s.repo.MarkDiplomaRevoked(ctx, vuzID, hash)
}

func (s *DiplomaService) HandleProcessingResult(ctx context.Context, result *model.KafkaProcessingResult) error {
	return s.repo.ApplyProcessingResult(ctx, result)
}
