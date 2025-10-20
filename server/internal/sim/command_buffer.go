package sim

import "sync"

const (
	commandBufferOccupancyMetricKey = "sim_command_buffer_occupancy"
	commandBufferOverflowMetricKey  = "sim_command_buffer_overflow_total"
)

// CommandBuffer stores staged commands in a fixed-size ring. It is safe for
// concurrent producers and a single consumer.
type CommandBuffer struct {
	mu      sync.Mutex
	data    []Command
	head    int
	tail    int
	count   int
	metrics telemetryMetrics
}

type telemetryMetrics interface {
	Add(string, uint64)
	Store(string, uint64)
}

// NewCommandBuffer constructs a ring buffer with the provided capacity.
func NewCommandBuffer(capacity int, metrics telemetryMetrics) *CommandBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &CommandBuffer{
		data:    make([]Command, capacity),
		metrics: metrics,
	}
}

// Capacity reports the maximum number of commands the buffer can hold.
func (b *CommandBuffer) Capacity() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.data)
}

// Push stages a command, returning false if the buffer is full.
func (b *CommandBuffer) Push(cmd Command) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count == len(b.data) {
		if b.metrics != nil {
			b.metrics.Add(commandBufferOverflowMetricKey, 1)
		}
		return false
	}
	b.data[b.tail] = cmd
	b.tail = (b.tail + 1) % len(b.data)
	b.count++
	b.storeOccupancyLocked()
	return true
}

// Drain returns all staged commands in FIFO order and clears the buffer.
func (b *CommandBuffer) Drain() []Command {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.count == 0 {
		return nil
	}
	commands := make([]Command, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.head + i) % len(b.data)
		commands[i] = b.data[idx]
	}
	b.head = 0
	b.tail = 0
	b.count = 0
	b.storeOccupancyLocked()
	return commands
}

// Len reports the number of staged commands.
func (b *CommandBuffer) Len() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

func (b *CommandBuffer) storeOccupancyLocked() {
	if b.metrics == nil {
		return
	}
	b.metrics.Store(commandBufferOccupancyMetricKey, uint64(b.count))
}
