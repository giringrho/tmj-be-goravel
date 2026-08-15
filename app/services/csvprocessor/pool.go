package csvprocessor

import "sync"

// Row represents a single CSV row, reused via sync.Pool to reduce GC pressure.
type Row struct {
	FileIndex int
	LineNo    int
	Fields    []string
}

// rowPool wraps sync.Pool for *Row reuse.
type rowPool struct {
	pool sync.Pool
}

func newRowPool() *rowPool {
	return &rowPool{
		pool: sync.Pool{
			New: func() any { return &Row{} },
		},
	}
}

// Get retrieves a Row from the pool, creating a new one if necessary.
func (p *rowPool) Get() *Row {
	return p.pool.Get().(*Row)
}

// Put returns a Row to the pool. Caller MUST nil out Fields before calling
// to avoid retaining references to CSV-internal buffers.
func (p *rowPool) Put(r *Row) {
	r.FileIndex = 0
	r.LineNo = 0
	r.Fields = nil
	p.pool.Put(r)
}
