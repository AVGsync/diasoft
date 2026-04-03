package excel

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/diasoft/gateway-service/internal/infrastructure/qr"
	"github.com/diasoft/gateway-service/internal/model"
	"github.com/xuri/excelize/v2"
)

type Generator struct {
	qrGenerator   *qr.Generator
	publicBaseURL string
}

func NewGenerator(qrGenerator *qr.Generator, publicBaseURL string) *Generator {
	return &Generator{
		qrGenerator:   qrGenerator,
		publicBaseURL: strings.TrimRight(publicBaseURL, "/"),
	}
}

func (g *Generator) BuildBatch(rows []*model.BatchDownloadRow) ([]byte, error) {
	file := excelize.NewFile()
	sheetName := "Diplomas"
	file.SetSheetName("Sheet1", sheetName)

	headers := []string{
		"Record Index",
		"Diploma Hash",
		"Full Name",
		"Diploma Number",
		"Specialty",
		"Degree",
		"Faculty",
		"Year",
		"Status",
		"Error",
		"QR",
	}

	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		if err := file.SetCellValue(sheetName, cell, header); err != nil {
			return nil, err
		}
	}

	if err := file.SetColWidth(sheetName, "A", "J", 24); err != nil {
		return nil, err
	}
	if err := file.SetColWidth(sheetName, "K", "K", 26); err != nil {
		return nil, err
	}

	for index, row := range rows {
		excelRow := index + 2
		values := []interface{}{
			row.RecordIndex,
			row.DiplomaHash,
			row.FullName,
			row.DiplomaNumber,
			row.Specialty,
			row.Degree,
			row.Faculty,
			row.Year,
			row.Status,
			valueOrEmpty(row.Error),
		}

		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, excelRow)
			if err := file.SetCellValue(sheetName, cell, value); err != nil {
				return nil, err
			}
		}

		if err := file.SetRowHeight(sheetName, excelRow, 110); err != nil {
			return nil, err
		}

		if row.QRPayload == "" {
			continue
		}

		qrURL := fmt.Sprintf("%s/api/v1/verify?payload=%s", g.publicBaseURL, url.QueryEscape(row.QRPayload))
		pngBytes, err := g.qrGenerator.PNG(qrURL, 180)
		if err != nil {
			return nil, err
		}

		cell, _ := excelize.CoordinatesToCellName(11, excelRow)
		if err := file.AddPictureFromBytes(sheetName, cell, &excelize.Picture{
			Extension: ".png",
			File:      pngBytes,
		}); err != nil {
			return nil, err
		}
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
