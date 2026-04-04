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
	sheetName := "Дипломы"
	file.SetSheetName("Sheet1", sheetName)

	headers := []string{
		"№ записи",
		"Хеш диплома",
		"ФИО",
		"Номер диплома",
		"Специальность",
		"Степень",
		"Факультет",
		"Год",
		"Статус",
		"Ошибка",
		"QR-код",
	}

	headerStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	if err != nil {
		return nil, err
	}

	bodyStyle, err := file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Vertical: "top",
			WrapText: true,
		},
	})
	if err != nil {
		return nil, err
	}

	centerStyle, err := file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	if err != nil {
		return nil, err
	}

	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		if err := file.SetCellValue(sheetName, cell, header); err != nil {
			return nil, err
		}
	}

	if err := file.SetCellStyle(sheetName, "A1", "K1", headerStyle); err != nil {
		return nil, err
	}
	if err := file.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, err
	}
	if err := file.SetRowHeight(sheetName, 1, 28); err != nil {
		return nil, err
	}

	columnWidths := map[string]float64{
		"A": 10,
		"B": 66,
		"C": 32,
		"D": 24,
		"E": 34,
		"F": 18,
		"G": 22,
		"H": 10,
		"I": 14,
		"J": 28,
		"K": 24,
	}
	for column, width := range columnWidths {
		if err := file.SetColWidth(sheetName, column, column, width); err != nil {
			return nil, err
		}
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

		if err := file.SetCellStyle(sheetName, fmt.Sprintf("A%d", excelRow), fmt.Sprintf("K%d", excelRow), bodyStyle); err != nil {
			return nil, err
		}
		if err := file.SetCellStyle(sheetName, fmt.Sprintf("A%d", excelRow), fmt.Sprintf("A%d", excelRow), centerStyle); err != nil {
			return nil, err
		}
		if err := file.SetCellStyle(sheetName, fmt.Sprintf("H%d", excelRow), fmt.Sprintf("K%d", excelRow), centerStyle); err != nil {
			return nil, err
		}

		if err := file.SetRowHeight(sheetName, excelRow, 128); err != nil {
			return nil, err
		}

		if row.QRPayload == "" {
			continue
		}

		qrURL := fmt.Sprintf("%s/verify?payload=%s", g.publicBaseURL, url.QueryEscape(row.QRPayload))
		pngBytes, err := g.qrGenerator.PNG(qrURL, 160)
		if err != nil {
			return nil, err
		}

		cell, _ := excelize.CoordinatesToCellName(11, excelRow)
		if err := file.AddPictureFromBytes(sheetName, cell, &excelize.Picture{
			Extension: ".png",
			File:      pngBytes,
			Format: &excelize.GraphicOptions{
				OffsetX:         10,
				OffsetY:         8,
				ScaleX:          0.78,
				ScaleY:          0.78,
				LockAspectRatio: true,
				Positioning:     "oneCell",
			},
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
