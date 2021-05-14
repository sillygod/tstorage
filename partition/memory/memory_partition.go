package memory

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nakabonne/tstorage/partition"
	"github.com/nakabonne/tstorage/wal"
)

var _ partition.MemoryPartition = &memoryPartition{}

// See NewMemoryPartition for details.
type memoryPartition struct {
	// A hash map from metric-name to metric.
	metrics sync.Map
	// Write ahead log.
	wal wal.WAL
	// The number of data points
	size int64
	// The timestamp range of partitions after which they get persisted
	partitionDuration int64

	minTimestamp int64
	maxTimestamp int64
}

// NewMemoryPartition generates a partition to store on the process memory.
func NewMemoryPartition(wal wal.WAL, partitionDuration time.Duration) partition.Partition {
	return &memoryPartition{
		partitionDuration: partitionDuration.Milliseconds(),
		wal:               wal,
	}
}

// InsertRows inserts the given rows to partition.
func (m *memoryPartition) InsertRows(rows []partition.Row) error {
	if len(rows) == 0 {
		return fmt.Errorf("no row was given")
	}
	if m.ReadOnly() {
		return fmt.Errorf("read only partition")
	}
	if m.wal != nil {
		m.wal.Append(wal.Entry{
			Operation: wal.OperationInsert,
			Rows:      rows,
		})
	}

	minTimestamp := rows[0].Timestamp
	maxTimestamp := rows[0].Timestamp
	var rowsNum int64
	for _, row := range rows {
		if row.Timestamp < minTimestamp {
			minTimestamp = row.Timestamp
		}
		if row.Timestamp > maxTimestamp {
			maxTimestamp = row.Timestamp
		}
		mt := m.getMetric(row.MetricName)
		mt.insertPoint(&row.DataPoint)
		rowsNum++
	}
	atomic.AddInt64(&m.size, rowsNum)

	// Make min/max timestamps up-to-date.
	if min := atomic.LoadInt64(&m.minTimestamp); min == 0 || min > minTimestamp {
		atomic.SwapInt64(&m.minTimestamp, minTimestamp)
	}
	if atomic.LoadInt64(&m.maxTimestamp) < maxTimestamp {
		atomic.SwapInt64(&m.maxTimestamp, maxTimestamp)
	}

	return nil
}

// SelectRows gives back the certain data points within the given range.
func (m *memoryPartition) SelectRows(metricName string, start, end int64) []partition.DataPoint {
	mt := m.getMetric(metricName)
	return mt.selectPoints(start, end)
}

// getMetric gives back the reference to the metrics list whose name is the given one.
// If none, it creates a new one.
func (m *memoryPartition) getMetric(name string) *metric {
	value, ok := m.metrics.Load(name)
	if !ok {
		value = &metric{
			name:      name,
			points:    make([]partition.DataPoint, 0),
			lastIndex: -1,
		}
		m.metrics.Store(name, value)
	}
	return value.(*metric)
}

func (m *memoryPartition) SelectAll() []partition.Row {
	rows := make([]partition.Row, 0, m.Size())
	m.metrics.Range(func(key, value interface{}) bool {
		mt, ok := value.(*metric)
		if !ok {
			return false
		}
		k, ok := key.(string)
		if !ok {
			return false
		}
		for _, point := range mt.points {
			rows = append(rows, partition.Row{
				MetricName: k,
				DataPoint: partition.DataPoint{
					Timestamp: point.Timestamp,
					Value:     point.Value,
				},
			})
		}
		return true
	})
	return rows
}

func (m *memoryPartition) ReadOnly() bool {
	return m.MaxTimestamp()-m.MinTimestamp() > m.partitionDuration
}

func (m *memoryPartition) MinTimestamp() int64 {
	return atomic.LoadInt64(&m.minTimestamp)
}

func (m *memoryPartition) MaxTimestamp() int64 {
	return atomic.LoadInt64(&m.maxTimestamp)
}

func (m *memoryPartition) Size() int {
	return int(atomic.LoadInt64(&m.size))
}

func (m *memoryPartition) ReadyToBePersisted() bool {
	return m.ReadOnly()
}
