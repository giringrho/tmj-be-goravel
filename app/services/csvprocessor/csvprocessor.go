// Package csvprocessor implements a concurrent CSV file processor using a
// worker-pool pattern (reader goroutines + builder workers) with fail-fast
// error handling, sync.Pool-based row reuse, and channel-based progress
// tracking.
//
// See docs/csvprocessor-design.md for the full design.
package csvprocessor

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

func defaultNumWorkers() int { return runtime.NumCPU() }

// ReadReport is sent by each reader goroutine to the collector. Err is non-nil
// when the file could not be opened or a read error occurred (fail-fast).
type ReadReport struct {
	FileIndex int
	FileName  string
	RowCount  int
	Err       error
}

// TotalReport aggregates per-file ReadReports into a final summary. This is
// the source of truth for the Result (tracker counters are only for progress).
type TotalReport struct {
	FilesTotal  int
	FilesRead   int
	RowsRead    int
	FailedFiles []string
}

// Result is the final output of Process.
type Result struct {
	Report TotalReport
	Series []TimeSeriesPoint
}

// Config controls Process behavior.
type Config struct {
	// NumWorkers is the number of builder goroutines. If <= 0, defaults to
	// runtime.NumCPU().
	NumWorkers int
	// ProgressCh optionally receives Progress events during processing.
	// Emission is non-blocking: events are dropped if the channel is full.
	// The caller is responsible for consuming from this channel; a buffered
	// channel is recommended. The channel is NOT closed by Process.
	ProgressCh chan Progress
}

// Process reads all given CSV files concurrently, parses rows with a worker
// pool, and aggregates created_at into a per-day time series.
//
// Fail-fast: the first error (file open, read, or row parse) cancels all
// in-flight work and is returned. On error, Result.Report is still populated
// with whatever was collected before the failure.
//
// ProgressCh is not closed by Process; the caller should drain it until a
// Progress{Done:true} event is received.
func Process(ctx context.Context, files []string, cfg Config) (Result, error) {
	if len(files) == 0 {
		return Result{Report: TotalReport{FilesTotal: 0}}, nil
	}

	numWorkers := cfg.NumWorkers
	if numWorkers <= 0 {
		numWorkers = defaultNumWorkers()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pool := newRowPool()
	trk := newTracker(int64(len(files)), cfg.ProgressCh)

	jobsCh := make(chan *Row, numWorkers*2)
	dataCh := make(chan MerchantData, numWorkers*2)
	readReportCh := make(chan ReadReport, len(files))
	errCh := make(chan error, 1) // first error wins; rest dropped

	// reportErr sends an error non-blocking. Subsequent errors are dropped.
	var firstErr atomic.Value // stores error
	reportErr := func(err error) {
		if err == nil {
			return
		}
		// Only the first error is kept; others dropped non-blocking.
		if firstErr.CompareAndSwap(nil, err) {
			select {
			case errCh <- err:
			default:
			}
		}
	}

	var workerReadWg sync.WaitGroup
	workerReadWg.Add(len(files))
	for i, name := range files {
		go func(i int, name string) {
			defer workerReadWg.Done()
			readCSVFile(ctx, i, name, pool, jobsCh, readReportCh, reportErr, trk)
		}(i, name)
	}

	var workerBuildWg sync.WaitGroup
	workerBuildWg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer workerBuildWg.Done()
			buildWorker(ctx, pool, jobsCh, dataCh, reportErr, trk)
		}()
	}

	// CLOSER 1: when all readers done, close jobsCh and readReportCh.
	go func() {
		workerReadWg.Wait()
		close(jobsCh)
		close(readReportCh)
	}()

	// CLOSER 2: when all builders done, close dataCh.
	go func() {
		workerBuildWg.Wait()
		close(dataCh)
	}()

	// COLLECTORS (concurrent, anti-deadlock).
	totalReport := TotalReport{FilesTotal: len(files)}
	seriesMap := make(map[string]int)
	var collectorWg sync.WaitGroup
	collectorWg.Add(2)

	go func() {
		defer collectorWg.Done()
		for r := range readReportCh {
			if r.Err != nil {
				totalReport.FailedFiles = append(totalReport.FailedFiles, r.FileName)
				continue
			}
			totalReport.FilesRead++
			totalReport.RowsRead += r.RowCount
		}
	}()

	go func() {
		defer collectorWg.Done()
		for d := range dataCh {
			seriesMap[d.CreatedAt.Format("2006-01-02")]++
		}
	}()

	collectorDone := make(chan struct{})
	go func() {
		collectorWg.Wait()
		close(collectorDone)
	}()

	// ERROR WATCHER: fail-fast on first error; otherwise exit on collectorDone.
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case err := <-errCh:
			if err != nil {
				cancel()
			}
		case <-collectorDone:
		}
	}()

	<-collectorDone
	<-watcherDone

	// Drain any error that landed in errCh after collectorDone (race safety).
	select {
	case err := <-errCh:
		firstErr.CompareAndSwap(nil, err)
	default:
	}

	var err error
	if v := firstErr.Load(); v != nil {
		err = v.(error)
	}

	trk.Finish(err)

	if err != nil {
		return Result{Report: totalReport}, err
	}
	return Result{Report: totalReport, Series: buildTimeSeries(seriesMap)}, nil
}
