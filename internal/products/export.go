// File: internal/products/export.go
// Usage: Builds the Excel (.xlsx) file for the products export.

package products

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

var exportHeaders = []string{
	"Code", "Name", "Currency", "Min amount", "Max amount", "Annual rate (%)", "Status", "Created at (UTC)",
}

// buildWorkbook renders products as an .xlsx file. Amounts are written as
// decimal strings so no floating point ever touches money values.
func buildWorkbook(items []Product) ([]byte, error) {
	const sheet = "Products"

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, fmt.Errorf("name sheet: %w", err)
	}
	if err := setRow(f, sheet, 1, toAnySlice(exportHeaders)); err != nil {
		return nil, err
	}
	for i, p := range items {
		row := []any{
			p.Code, p.Name, p.Currency,
			p.MinAmount.StringFixed(2), p.MaxAmount.StringFixed(2), p.AnnualRate.StringFixed(2),
			string(p.Status), p.CreatedAt.UTC().Format(time.DateTime),
		}
		if err := setRow(f, sheet, i+2, row); err != nil {
			return nil, err
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// setRow writes one row of values into the sheet, starting at column A.
func setRow(f *excelize.File, sheet string, rowNum int, values []any) error {
	cell, err := excelize.CoordinatesToCellName(1, rowNum)
	if err != nil {
		return fmt.Errorf("cell name: %w", err)
	}
	if err := f.SetSheetRow(sheet, cell, &values); err != nil {
		return fmt.Errorf("write row %d: %w", rowNum, err)
	}
	return nil
}

// toAnySlice converts []string to []any, which excelize expects.
func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
