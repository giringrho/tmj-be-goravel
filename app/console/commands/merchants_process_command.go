package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"

	"goravel/app/services/csvprocessor"
)

// MerchantsProcessCommand processes all CSV files in a directory using the
// concurrent csvprocessor and prints a summary + time-series table.
//
// Usage: ./artisan merchants:process <dir> [--workers=4]
type MerchantsProcessCommand struct{}

func NewMerchantsProcessCommand() *MerchantsProcessCommand {
	return &MerchantsProcessCommand{}
}

func (r *MerchantsProcessCommand) Signature() string { return "merchants:process" }

func (r *MerchantsProcessCommand) Description() string {
	return "Process Merchant CSV files in a directory and aggregate per-day counts"
}

func (r *MerchantsProcessCommand) Extend() command.Extend {
	return command.Extend{
		Category:  "merchants",
		ArgsUsage: "<dir>  directory containing *.csv files",
		Flags: []command.Flag{
			&command.IntFlag{
				Name:  "workers",
				Usage: "number of builder workers",
				Value: 4,
			},
		},
	}
}

func (r *MerchantsProcessCommand) Handle(ctx console.Context) error {
	dir := ctx.Argument(0)
	if dir == "" {
		ctx.Error("directory argument is required")
		return fmt.Errorf("missing directory argument")
	}

	workers := ctx.OptionInt("workers")
	if workers <= 0 {
		workers = 4
	}

	// Collect CSV files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		ctx.Error(fmt.Sprintf("read dir %s: %v", dir, err))
		return err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) == 0 {
		ctx.Warning(fmt.Sprintf("no CSV files found in %s", dir))
		return nil
	}

	ctx.Info(fmt.Sprintf("Processing %d file(s) with %d workers...", len(files), workers))
	for _, f := range files {
		ctx.Line("  " + f)
	}
	ctx.NewLine()

	// Progress channel with a small buffer; drain in a goroutine and print
	// compact progress lines.
	progressCh := make(chan csvprocessor.Progress, 16)
	go func() {
		for p := range progressCh {
			if p.Done {
				return
			}
			ctx.Line(fmt.Sprintf("  files: %d/%d  rows read: %d  processed: %d",
				p.FilesRead, p.FilesTotal, p.RowsRead, p.RowsProcessed))
		}
	}()

	res, err := csvprocessor.Process(context.Background(), files, csvprocessor.Config{
		NumWorkers: workers,
		ProgressCh: progressCh,
	})
	close(progressCh)

	if err != nil {
		ctx.Error(fmt.Sprintf("Processing failed: %v", err))
		if len(res.Report.FailedFiles) > 0 {
			ctx.Line("Failed files:")
			for _, f := range res.Report.FailedFiles {
				ctx.Redln("  " + f)
			}
		}
		return err
	}

	// Summary
	ctx.Success("Processing complete")
	ctx.NewLine()
	ctx.TwoColumnDetail("Files total", fmt.Sprintf("%d", res.Report.FilesTotal))
	ctx.TwoColumnDetail("Files read", fmt.Sprintf("%d", res.Report.FilesRead))
	ctx.TwoColumnDetail("Rows read", fmt.Sprintf("%d", res.Report.RowsRead))
	if len(res.Report.FailedFiles) > 0 {
		ctx.TwoColumnDetail("Failed files", fmt.Sprintf("%d", len(res.Report.FailedFiles)))
	}
	ctx.NewLine()

	// Time-series table
	ctx.Info("Time series (merchants per day):")
	ctx.NewLine()
	headers := []string{"Date", "Count"}
	rows := make([][]string, 0, len(res.Series))
	total := 0
	for _, p := range res.Series {
		rows = append(rows, []string{p.Date, fmt.Sprintf("%d", p.Count)})
		total += p.Count
	}
	rows = append(rows, []string{"TOTAL", fmt.Sprintf("%d", total)})
	ctx.Table(headers, rows)

	return nil
}
