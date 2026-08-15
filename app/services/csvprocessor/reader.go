package csvprocessor

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
)

// readCSVFile reads one CSV file and sends each data row (header skipped)
// to jobsCh. On success it sends a ReadReport to readReportCh. On open or
// read error it sends a ReadReport{Err: err} so FailedFiles is tracked,
// reports the error for fail-fast, and returns without sending a success
// report.
func readCSVFile(
	ctx context.Context,
	fileIndex int,
	fileName string,
	pool *rowPool,
	jobsCh chan<- *Row,
	readReportCh chan<- ReadReport,
	reportErr func(error),
	trk *tracker,
) {
	f, err := os.Open(fileName)
	if err != nil {
		// Send a failure report so collector A can record FailedFiles.
		sendReport(ctx, readReportCh, ReadReport{FileIndex: fileIndex, FileName: fileName, Err: err})
		reportErr(fmt.Errorf("open %s: %w", fileName, err))
		return
	}
	defer f.Close()

	r := csv.NewReader(f)
	rowCount := 0
	lineNo := 0 // 1 = header, data rows start at lineNo 2

	for {
		if err := ctx.Err(); err != nil {
			// Context cancelled: stop reading, do not send a success report.
			return
		}
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, csv.ErrFieldCount) {
				// Wrong field count → fail-fast.
				lineNo++
				sendReport(ctx, readReportCh, ReadReport{
					FileIndex: fileIndex, FileName: fileName, RowCount: rowCount,
					Err: fmt.Errorf("%s line %d: %w", fileName, lineNo, err),
				})
				reportErr(fmt.Errorf("%s line %d: %w", fileName, lineNo, err))
				return
			}
			if err == io.EOF {
				break // clean EOF
			}
			// Any other read error → fail-fast.
			sendReport(ctx, readReportCh, ReadReport{
				FileIndex: fileIndex, FileName: fileName, RowCount: rowCount,
				Err: fmt.Errorf("read %s: %w", fileName, err),
			})
			reportErr(fmt.Errorf("read %s: %w", fileName, err))
			return
		}

		lineNo++
		if lineNo == 1 {
			continue // skip header row
		}

		row := pool.Get()
		row.FileIndex = fileIndex
		row.LineNo = lineNo
		// Copy the record: csv.Reader reuses its underlying slice on next Read,
		// so we must not retain the original. A shallow copy of the slice header
		// is insufficient; allocate a new backing array.
		row.Fields = append(row.Fields[:0:0], rec...)

		select {
		case jobsCh <- row:
		case <-ctx.Done():
			pool.Put(row)
			return
		}
		rowCount++
		trk.incRowsRead()
	}

	sendReport(ctx, readReportCh, ReadReport{
		FileIndex: fileIndex, FileName: fileName, RowCount: rowCount,
	})
	trk.incFilesRead()
}

// sendReport sends a ReadReport to readReportCh, respecting context cancellation.
func sendReport(ctx context.Context, ch chan<- ReadReport, r ReadReport) {
	select {
	case ch <- r:
	case <-ctx.Done():
	}
}
