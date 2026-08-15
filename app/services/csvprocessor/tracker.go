package csvprocessor

import (
	"sync/atomic"
)

// Progress is emitted to Config.ProgressCh during processing.
// Emission is non-blocking: if the channel is full, the event is dropped.
type Progress struct {
	FilesTotal    int64
	FilesRead     int64
	RowsRead      int64
	RowsProcessed int64
	Done          bool
	Err           error
}

// progressEmitInterval controls how often Progress is emitted during row
// processing (every N rows). A final Progress{Done:true} is always emitted.
const progressEmitInterval = 100

// tracker holds atomic counters and emits Progress events to a channel.
// It is purely for progress reporting; TotalReport (collected from
// ReadReport) is the source of truth for the final Result.
type tracker struct {
	filesTotal    atomic.Int64
	filesRead     atomic.Int64
	rowsRead      atomic.Int64
	rowsProcessed atomic.Int64
	progressCh    chan Progress
}

func newTracker(filesTotal int64, progressCh chan Progress) *tracker {
	t := &tracker{progressCh: progressCh}
	t.filesTotal.Store(filesTotal)
	return t
}

func (t *tracker) incFilesRead() { t.filesRead.Add(1) }

func (t *tracker) incRowsRead() { t.rowsRead.Add(1) }

// incRowsProcessed increments the counter and emits a Progress event
// every progressEmitInterval rows (non-blocking).
func (t *tracker) incRowsProcessed() {
	n := t.rowsProcessed.Add(1)
	if n%progressEmitInterval == 0 {
		t.emit(false, nil)
	}
}

// emit sends a Progress event non-blocking. Drops the event if the channel
// is full or nil, so workers are never blocked by a slow consumer.
func (t *tracker) emit(done bool, err error) {
	if t.progressCh == nil {
		return
	}
	select {
	case t.progressCh <- Progress{
		FilesTotal:    t.filesTotal.Load(),
		FilesRead:     t.filesRead.Load(),
		RowsRead:      t.rowsRead.Load(),
		RowsProcessed: t.rowsProcessed.Load(),
		Done:          done,
		Err:           err,
	}:
	default:
	}
}

// Finish emits the final Progress{Done:true} event.
func (t *tracker) Finish(err error) { t.emit(true, err) }
