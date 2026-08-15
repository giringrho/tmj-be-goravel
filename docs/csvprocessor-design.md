# CSV Concurrent Processor — Design

## Tujuan

Concurrent file processor untuk membaca multiple CSV Merchants, memproses dengan worker pool, dan menghasilkan time-series per hari untuk visualisasi.

## Kebutuhan

1. Membaca multiple CSV file secara simultan
2. Worker pool pattern (reader + builder)
3. Error handling fail-fast
4. Progress tracking via channel events
5. Memory management via `sync.Pool` untuk `*Row`

## Schema CSV (Merchants)

Header: `id,user_id,merchant_name,created_at,created_by,updated_at,updated_by`

```sql
CREATE TABLE `Merchants` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `user_id` int(40) NOT NULL,
  `merchant_name` varchar(40) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_by` bigint(20) NOT NULL,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_by` bigint(20) NOT NULL,
  PRIMARY KEY (`id`)
);
```

## Lokasi Code

```
app/services/csvprocessor/
├── csvprocessor.go   # types + public Process() API
├── tracker.go        # Tracker (atomics + Progress channel emit)
├── pool.go           # sync.Pool wrapper for *Row
├── reader.go         # readCSVFile goroutine
├── builder.go        # buildWorker goroutine + processRow
├── aggregate.go      # buildTimeSeries (map → sorted slice)
├── testdata/
│   ├── merchants_1.csv
│   ├── merchants_2.csv
│   ├── merchants_3.csv
│   └── corrupt.csv
└── csvprocessor_test.go
```

## Types

```go
type Row struct {
    FileIndex int
    LineNo    int
    Fields    []string
}

type MerchantData struct {
    ID           int64
    UserID       int64
    MerchantName string
    CreatedAt    time.Time
    CreatedBy    int64
    UpdatedAt    time.Time
    UpdatedBy    int64
}

type ReadReport struct {
    FileIndex int
    FileName  string
    RowCount  int
    Err       error
}

type TotalReport struct {
    FilesTotal  int
    FilesRead   int
    RowsRead    int
    FailedFiles []string
}

type Progress struct {
    FilesTotal    int64
    FilesRead     int64
    RowsRead      int64
    RowsProcessed int64
    Done          bool
    Err           error
}

type TimeSeriesPoint struct {
    Date  string // YYYY-MM-DD
    Count int
}

type Result struct {
    Report TotalReport
    Series []TimeSeriesPoint
}

type Config struct {
    NumWorkers int
    ProgressCh chan Progress // optional
}
```

## Public API

```go
func Process(ctx context.Context, files []string, cfg Config) (Result, error)
```

## Flow (revisi v2)

```
Process(ctx, files, cfg)
│
├── ctx, cancel := WithCancel(ctx)
├── errCh := chan error (buffer 1)        // first error wins, rest dropped
├── reportErr(err): non-blocking send ke errCh (select + default: drop)
│
├── READERS (N = len(files)) ─── workerReadWg
│     per reader:
│       defer workerReadWg.Done()
│       open file → err?
│         reportErr(err)
│         readReportCh <- ReadReport{FileIndex, FileName, Err: err}  // kirim report gagal
│         return
│       csv.Reader loop, lineNo mulai dari 2 (skip baris 1 = header):
│         select ctx.Done() → return
│         row := pool.Get(); fill Fields (FileIndex, LineNo, Fields)
│         jobsCh <- row          (select ctx.Done() + default backpressure)
│         tracker.incRowsRead
│       readReportCh <- ReadReport{rowCount, nil}
│       tracker.incFilesRead
│
├── BUILDERS (M = numWorkers) ─── workerBuildWg
│     per builder:
│       defer workerBuildWg.Done()
│       for row := range jobsCh:
│         select ctx.Done() → return
│         data, err := processRow(row)
│         if err != nil:
│           row.Fields = nil; pool.Put(row)   // pool.Put di SEMUA path
│           reportErr(err); return
│         dataCh <- data
│         tracker.incRowsProcessed
│         row.Fields = nil; pool.Put(row)
│
├── CLOSER 1: workerReadWg.Wait() → close(jobsCh), close(readReportCh)
├── CLOSER 2: workerBuildWg.Wait() → close(dataCh)
│
├── COLLECTORS (concurrent, 2 goroutine + collectorWg):
│     A: range readReportCh →
│          if report.Err != nil → totalReport.FailedFiles append
│          else → totalReport.FilesRead++; totalReport.RowsRead += report.RowCount
│     B: range dataCh → seriesMap[YYYY-MM-DD(createdAt)]++
│     collectorWg.Wait() → close(collectorDone)
│
├── ERROR WATCHER (goroutine, exit ditunggu via watcherDone):
│     select:
│       err := <-errCh → firstErr = err (Once); cancel()    // fail-fast
│       <-collectorDone → return
│     close(watcherDone)
│
├── <-collectorDone
├── <-watcherDone                          // pastikan watcher exit
├── drain errCh non-blocking (select + default), merge ke firstErr via Once
├── tracker.Finish() → emit Progress{Done:true, Err:firstErr} (non-blocking)
├── if firstErr != nil → return Result{Report: totalReport}, firstErr
└── return Result{Report: totalReport, Series: buildTimeSeries(seriesMap)}, nil
```

## Key Points

1. **Anti-deadlock**: collector A & B concurrent → tidak ada blocking antara report & data stream
2. **Fail-fast**: error watcher langsung `cancel()` saat error pertama; reader/builder exit via `ctx.Done()`
3. **Pool discipline**: `row.Fields = nil` sebelum `pool.Put` di SEMUA path (success + error) agar tidak menahan referensi memori CSV
4. **Channel buffer**: `jobsCh`/`dataCh` = `numWorkers*2`, `readReportCh` = `len(files)`, `errCh` = 1
5. **Tracker**: `atomic.Int64` counters; emit `Progress` ke channel **non-blocking** (select + default: drop) supaya worker tidak ter-block oleh consumer lambat. Throttle: emit tiap 100 rows processed + saat `Finish()`
6. **processRow**: parse 7 field → `MerchantData` dengan validasi (fail-fast on parse error / wrong field count / bad timestamp). Layout `created_at` & `updated_at`: `2006-01-02 15:04:05`
7. **Header skip**: reader skip baris pertama (header), `lineNo` mulai dari 2
8. **FailedFiles tracking**: reader kirim `ReadReport{Err: err}` saat open gagal → collector A isi `totalReport.FailedFiles`
9. **Source of truth**: `TotalReport` (dari collector A) = sumber data final; tracker hanya untuk progress event, tidak dipakai untuk Result
10. **Watcher sync**: `Process` menunggu `<-watcherDone` + drain `errCh` setelah `collectorDone` → tidak ada error yang hilang saat race akhir

## Dummy CSV

- `merchants_1.csv` — 1500 rows, `created_at` tersebar 7 hari
- `merchants_2.csv` — 2000 rows
- `merchants_3.csv` — 1200 rows
- `corrupt.csv` — field count tidak konsisten, untuk test fail-fast
- Total valid: 4700 rows → time-series 7 hari

## Test Plan

- `TestProcessRow` — parsing valid & invalid row (wrong field count, bad timestamp)
- `TestProcess_HappyPath` — 3 file, assert total rows = 4700, series terurut, progress events ter-emit, header tidak ikut diproses
- `TestProcess_FailFast` — corrupt file → return error, ctx cancelled, no hang, `FailedFiles` terisi
- `TestProcess_Cancel` — parent ctx cancel mid-process → clean shutdown
- `TestProcess_EmptyFiles` — edge case (0 file, file kosong)
- `TestProcess_OpenError` — file tidak ada → `ReadReport{Err}` terkirim, `FailedFiles` terisi, fail-fast terpicu
- `TestTracker` — atomic counters, throttle tiap 100 rows, non-blocking emit
- `TestBuildTimeSeries` — sorting & aggregation
- `TestPool` — `*Row` direuse, `Fields` di-nil sebelum Put (no memory leak)

## Goravel Integration (next phase)

- Controller endpoint `POST /merchants/process` terima file upload → call `csvprocessor.Process`
- Progress events bisa di-stream via SSE/WebSocket
- Bisa dibungkus console command `./artisan merchants:process <dir>`
