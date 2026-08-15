package csvprocessor

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// timestampLayout matches the MySQL TIMESTAMP default format used in the
// Merchants CSV (created_at, updated_at).
const timestampLayout = "2006-01-02 15:04:05"

// expectedFieldCount is the number of columns in the Merchants CSV.
const expectedFieldCount = 7

// MerchantData is the parsed representation of one Merchants CSV row.
type MerchantData struct {
	ID           int64
	UserID       int64
	MerchantName string
	CreatedAt    time.Time
	CreatedBy    int64
	UpdatedAt    time.Time
	UpdatedBy    int64
}

// processRow parses a raw CSV row into MerchantData. Returns an error on
// wrong field count or parse failure (fail-fast).
func processRow(row *Row) (MerchantData, error) {
	var m MerchantData
	if len(row.Fields) != expectedFieldCount {
		return m, fmt.Errorf("file %d line %d: expected %d fields, got %d",
			row.FileIndex, row.LineNo, expectedFieldCount, len(row.Fields))
	}

	f := row.Fields
	var err error

	if m.ID, err = parseInt64(f[0], row, "id"); err != nil {
		return m, err
	}
	if m.UserID, err = parseInt64(f[1], row, "user_id"); err != nil {
		return m, err
	}
	m.MerchantName = f[2]
	if m.CreatedAt, err = parseTime(f[3], row, "created_at"); err != nil {
		return m, err
	}
	if m.CreatedBy, err = parseInt64(f[4], row, "created_by"); err != nil {
		return m, err
	}
	if m.UpdatedAt, err = parseTime(f[5], row, "updated_at"); err != nil {
		return m, err
	}
	if m.UpdatedBy, err = parseInt64(f[6], row, "updated_by"); err != nil {
		return m, err
	}
	return m, nil
}

func parseInt64(s string, row *Row, field string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("file %d line %d: parse %s %q: %w",
			row.FileIndex, row.LineNo, field, s, err)
	}
	return v, nil
}

func parseTime(s string, row *Row, field string) (time.Time, error) {
	t, err := time.Parse(timestampLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("file %d line %d: parse %s %q: %w",
			row.FileIndex, row.LineNo, field, s, err)
	}
	return t, nil
}

// buildWorker pulls rows from jobsCh, parses them, and sends MerchantData to
// dataCh. On error it reports for fail-fast and returns. The Row is returned
// to the pool on every path.
func buildWorker(
	ctx context.Context,
	pool *rowPool,
	jobsCh <-chan *Row,
	dataCh chan<- MerchantData,
	reportErr func(error),
	trk *tracker,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case row, ok := <-jobsCh:
			if !ok {
				return
			}
			data, err := processRow(row)
			if err != nil {
				pool.Put(row)
				reportErr(err)
				return
			}
			select {
			case dataCh <- data:
			case <-ctx.Done():
				pool.Put(row)
				return
			}
			trk.incRowsProcessed()
			pool.Put(row)
		}
	}
}
