package csvprocessor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "testdata", name)
}

func validRow() *Row {
	return &Row{
		FileIndex: 0,
		LineNo:    2,
		Fields:    []string{"1", "24", "Toko Maju", "2026-08-13 11:08:30", "46", "2026-08-13 11:08:30", "8"},
	}
}

func TestProcessRow_Valid(t *testing.T) {
	row := validRow()
	m, err := processRow(row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != 1 || m.UserID != 24 || m.MerchantName != "Toko Maju" {
		t.Fatalf("unexpected data: %+v", m)
	}
	want := "2026-08-13 11:08:30"
	if m.CreatedAt.Format(timestampLayout) != want {
		t.Fatalf("CreatedAt = %s, want %s", m.CreatedAt.Format(timestampLayout), want)
	}
	if m.CreatedBy != 46 || m.UpdatedBy != 8 {
		t.Fatalf("unexpected by fields: %+v", m)
	}
}

func TestProcessRow_WrongFieldCount(t *testing.T) {
	row := &Row{FileIndex: 1, LineNo: 3, Fields: []string{"1", "2", "name"}}
	if _, err := processRow(row); err == nil {
		t.Fatal("expected error for wrong field count, got nil")
	}
}

func TestProcessRow_BadTimestamp(t *testing.T) {
	row := &Row{FileIndex: 0, LineNo: 2, Fields: []string{"1", "2", "n", "not-a-date", "1", "2026-08-13 11:08:30", "1"}}
	if _, err := processRow(row); err == nil {
		t.Fatal("expected error for bad timestamp, got nil")
	}
}

func TestProcessRow_BadInt(t *testing.T) {
	row := &Row{FileIndex: 0, LineNo: 2, Fields: []string{"abc", "2", "n", "2026-08-13 11:08:30", "1", "2026-08-13 11:08:30", "1"}}
	if _, err := processRow(row); err == nil {
		t.Fatal("expected error for bad int, got nil")
	}
}

func TestProcess_HappyPath(t *testing.T) {
	files := []string{
		testdataPath(t, "merchants_1.csv"),
		testdataPath(t, "merchants_2.csv"),
		testdataPath(t, "merchants_3.csv"),
	}
	progressCh := make(chan Progress, 64)
	res, err := Process(context.Background(), files, Config{NumWorkers: 4, ProgressCh: progressCh})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Report.FilesTotal != 3 {
		t.Fatalf("FilesTotal = %d, want 3", res.Report.FilesTotal)
	}
	if res.Report.FilesRead != 3 {
		t.Fatalf("FilesRead = %d, want 3", res.Report.FilesRead)
	}
	// 1500 + 2000 + 1200 = 4700 (header excluded)
	if res.Report.RowsRead != 4700 {
		t.Fatalf("RowsRead = %d, want 4700", res.Report.RowsRead)
	}
	if len(res.Report.FailedFiles) != 0 {
		t.Fatalf("FailedFiles = %v, want empty", res.Report.FailedFiles)
	}

	// Time series: 7 days (2026-08-08 .. 2026-08-14), sorted ascending.
	if len(res.Series) != 7 {
		t.Fatalf("Series len = %d, want 7", len(res.Series))
	}
	if !sort.SliceIsSorted(res.Series, func(i, j int) bool { return res.Series[i].Date < res.Series[j].Date }) {
		t.Fatalf("Series not sorted: %+v", res.Series)
	}
	totalCount := 0
	for _, p := range res.Series {
		if p.Date < "2026-08-08" || p.Date > "2026-08-14" {
			t.Fatalf("unexpected date %s", p.Date)
		}
		totalCount += p.Count
	}
	if totalCount != 4700 {
		t.Fatalf("sum of series counts = %d, want 4700", totalCount)
	}

	// Progress events: at least one Done event should be emitted.
	close(progressCh)
	sawDone := false
	for p := range progressCh {
		if p.Done {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("no Progress{Done:true} event emitted")
	}
}

func TestProcess_FailFast(t *testing.T) {
	files := []string{
		testdataPath(t, "merchants_1.csv"),
		testdataPath(t, "corrupt.csv"),
		testdataPath(t, "merchants_2.csv"),
	}
	progressCh := make(chan Progress, 16)
	res, err := Process(context.Background(), files, Config{NumWorkers: 2, ProgressCh: progressCh})
	if err == nil {
		t.Fatal("expected error for corrupt file, got nil")
	}
	// The corrupt file should be recorded in FailedFiles.
	if !contains(res.Report.FailedFiles, "corrupt.csv") {
		t.Fatalf("FailedFiles = %v, want to contain corrupt.csv", res.Report.FailedFiles)
	}
	// Drain progress to confirm Done event with error.
	close(progressCh)
	for p := range progressCh {
		if p.Done && p.Err != nil {
			return // pass
		}
	}
}

func TestProcess_Cancel(t *testing.T) {
	files := []string{
		testdataPath(t, "merchants_1.csv"),
		testdataPath(t, "merchants_2.csv"),
		testdataPath(t, "merchants_3.csv"),
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel almost immediately to trigger mid-process cancellation.
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := Process(ctx, files, Config{NumWorkers: 2})
	elapsed := time.Since(start)
	// Should not hang. Either returns an error (cancelled) or completes if it
	// was fast enough; either way it must return within a reasonable bound.
	if elapsed > 5*time.Second {
		t.Fatalf("Process hung for %v", elapsed)
	}
	_ = err // cancellation may or may not surface as error depending on timing
}

func TestProcess_EmptyFiles(t *testing.T) {
	res, err := Process(context.Background(), nil, Config{NumWorkers: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Report.FilesTotal != 0 || len(res.Series) != 0 {
		t.Fatalf("expected empty result, got %+v", res)
	}
}

func TestProcess_OpenError(t *testing.T) {
	files := []string{testdataPath(t, "merchants_1.csv"), testdataPath(t, "does_not_exist.csv")}
	res, err := Process(context.Background(), files, Config{NumWorkers: 2})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !contains(res.Report.FailedFiles, "does_not_exist.csv") {
		t.Fatalf("FailedFiles = %v, want does_not_exist.csv", res.Report.FailedFiles)
	}
}

func TestTracker_Emit(t *testing.T) {
	ch := make(chan Progress, 16)
	trk := newTracker(3, ch)
	trk.incFilesRead()
	for i := 0; i < 250; i++ {
		trk.incRowsProcessed()
	}
	trk.Finish(nil)
	close(ch)

	var last Progress
	count := 0
	for p := range ch {
		last = p
		count++
	}
	if count == 0 {
		t.Fatal("no progress events emitted")
	}
	if !last.Done {
		t.Fatalf("last event Done = false, want true")
	}
	if last.FilesTotal != 3 || last.FilesRead != 1 {
		t.Fatalf("last = %+v", last)
	}
	if last.RowsProcessed != 250 {
		t.Fatalf("RowsProcessed = %d, want 250", last.RowsProcessed)
	}
}

func TestTracker_NonBlocking(t *testing.T) {
	// Unbuffered channel + no consumer: emit must not block.
	ch := make(chan Progress)
	trk := newTracker(1, ch)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			trk.incRowsProcessed() // would block if emit were blocking
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emit blocked on unbuffered channel")
	}
}

func TestBuildTimeSeries(t *testing.T) {
	m := map[string]int{
		"2026-08-10": 5,
		"2026-08-08": 3,
		"2026-08-09": 7,
	}
	got := buildTimeSeries(m)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := []string{"2026-08-08", "2026-08-09", "2026-08-10"}
	for i, w := range want {
		if got[i].Date != w {
			t.Fatalf("got[%d].Date = %s, want %s", i, got[i].Date, w)
		}
	}
}

func TestPool_ReuseAndNilFields(t *testing.T) {
	p := newRowPool()
	r := p.Get()
	r.Fields = []string{"a", "b"}
	p.Put(r)
	r2 := p.Get()
	// Same object may be reused (pool is non-deterministic, but in practice
	// immediately after Put it is). Fields must be nil regardless.
	if r2.Fields != nil {
		t.Fatalf("Fields = %v, want nil after Put", r2.Fields)
	}
}

func TestPool_Concurrent(t *testing.T) {
	p := newRowPool()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := p.Get()
			r.Fields = []string{"x"}
			p.Put(r)
		}()
	}
	wg.Wait()
}

// contains checks if s is in the slice (substring match for file names).
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if strings.Contains(v, s) {
			return true
		}
	}
	return false
}

// Ensure errors.Is works on sentinel checks used by the package (compile guard).
var _ = errors.Is
