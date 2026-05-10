package monarch

import (
	"encoding/csv"
	"fmt"
	"io"
)

type csvWriter interface {
	Write(record []string) error
	Flush()
	Error() error
}

func ExportTransactionsCSV(txs []Transaction, w io.Writer) error {
	return exportTransactionsCSV(txs, w, func(w io.Writer) csvWriter {
		return csv.NewWriter(w)
	})
}

func exportTransactionsCSV(txs []Transaction, w io.Writer, newWriter func(io.Writer) csvWriter) error {
	writer := newWriter(w)

	header := []string{"Date", "Merchant", "Category", "Amount", "Notes"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, t := range txs {
		row := []string{
			t.Date,
			t.Merchant,
			t.Category,
			fmt.Sprintf("%.2f", t.Amount),
			t.Notes,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return nil
}
