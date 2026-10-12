// Package workerpool runs tasks on a fixed set of worker goroutines, the
// analog of java.util.concurrent.ThreadPoolExecutor with a bounded queue.
//
// It is the bounded-concurrency primitive behind "run at most N of these at a
// time" work (frame jobs, batch uploads, background I/O): Submit applies
// backpressure when the queue is full instead of growing without bound, and a
// panicking task is contained instead of taking the process down.
//
//	p := workerpool.New(4, workerpool.WithQueueSize(1024))
//	defer p.Close()
//
//	p.Submit(func() { doWork() })     // blocks while the queue is full
//	p.TrySubmit(func() { doWork() })  // reports ErrQueueFull instead
//	p.SubmitWait(func() { doWork() }) // waits for the task to finish
//
// Pool is safe for concurrent use.
package workerpool

import (
	"errors"
	"sync"
	"sync/atomic"
)

// defaultQueueSize is the pending-task queue capacity used when no
// WithQueueSize option is given.
const defaultQueueSize = 1024

// Task is a unit of work executed by a worker goroutine.
type Task func()

// Option configures a Pool.
type Option func(*Pool)

// ErrClosed is returned by Submit/TrySubmit/SubmitWait once the pool is closed.
var ErrClosed = errors.New("workerpool: pool is closed")

// ErrQueueFull is returned by TrySubmit when no queue slot is free.
var ErrQueueFull = errors.New("workerpool: queue is full")

// WithQueueSize sets the capacity of the pending-task queue. 0 means an
// unbuffered hand-off (Submit blocks until a worker picks the task up).
// Default: 1024. Panics at New time when n < 0.
func WithQueueSize(n int) Option {
	return func(p *Pool) {
		if n < 0 {
			panic("workerpool: queue size must not be negative")
		}
		p.queueCap = n
	}
}

// WithPanicHandler sets the function called with the recovered value when a
// task panics. By default panics are only counted in Metrics.Panics; a nil
// handler keeps that default.
func WithPanicHandler(fn func(recovered any)) Option {
	return func(p *Pool) { p.onPanic = fn }
}

// Metrics is a snapshot of the pool counters.
type Metrics struct {
	// Workers is the number of worker goroutines.
	Workers int
	// Queued is the number of tasks waiting in the queue right now.
	Queued int
	// QueueCap is the queue capacity (0 for an unbuffered queue).
	QueueCap int
	// Active is the number of workers currently running a task.
	Active int
	// Submitted is the number of tasks accepted by the pool.
	Submitted int64
	// Completed is the number of tasks that finished (ran or panicked).
	Completed int64
	// Panics is the number of tasks that panicked.
	Panics int64
}

// Pool runs tasks on a fixed set of worker goroutines.
type Pool struct {
	// mu guards closed and the close of tasks. Submit and TrySubmit hold the
	// read lock across the channel send, so the channel can never be closed
	// under a sender; Close takes the write lock to close it.
	mu     sync.RWMutex
	tasks  chan Task
	closed bool

	closeOnce sync.Once
	wg        sync.WaitGroup

	// immutable after New
	workers  int
	queueCap int
	onPanic  func(recovered any)

	active    atomic.Int64
	submitted atomic.Int64
	completed atomic.Int64
	panics    atomic.Int64
}

// New creates a pool with the given number of worker goroutines (> 0).
// It panics when workers <= 0 or an option is invalid.
func New(workers int, opts ...Option) *Pool {
	if workers <= 0 {
		panic("workerpool: workers must be positive")
	}
	p := &Pool{workers: workers, queueCap: defaultQueueSize}
	for _, opt := range opts {
		opt(p)
	}
	p.tasks = make(chan Task, p.queueCap)
	p.wg.Add(workers)
	for range workers {
		go p.worker()
	}
	return p
}

// Submit queues task, blocking while the queue is full (backpressure).
// It returns ErrClosed when the pool is closed.
func (p *Pool) Submit(task Task) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	// the send cannot block on a closed channel: Close needs the write lock
	p.submitted.Add(1)
	p.tasks <- task
	return nil
}

// TrySubmit queues task without blocking. It returns ErrQueueFull when no
// slot is free and ErrClosed when the pool is closed.
func (p *Pool) TrySubmit(task Task) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.tasks <- task:
		p.submitted.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// SubmitWait queues task and blocks until it has finished running.
// It returns ErrClosed when the pool is closed before accepting the task.
func (p *Pool) SubmitWait(task Task) error {
	done := make(chan struct{})
	err := p.Submit(func() {
		defer close(done)
		task()
	})
	if err != nil {
		return err
	}
	<-done
	return nil
}

// Close stops accepting tasks, lets the queued and in-flight tasks finish,
// and returns once every worker has exited. It is idempotent and safe to call
// from several goroutines.
//
// Close takes the write lock, so it waits for a Submit that is blocked on a
// full queue: that submitter holds the read lock until its task is handed
// over, and the workers keep draining the queue while Close waits, so the
// hand-off always completes instead of being dropped. Submissions that arrive
// after Close report ErrClosed.
func (p *Pool) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.tasks)
		p.mu.Unlock()
	})
	p.wg.Wait()
}

// Closed reports whether Close has been called.
func (p *Pool) Closed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

// Metrics returns a snapshot of the pool counters.
func (p *Pool) Metrics() Metrics {
	return Metrics{
		Workers:   p.workers,
		Queued:    len(p.tasks),
		QueueCap:  p.queueCap,
		Active:    int(p.active.Load()),
		Submitted: p.submitted.Load(),
		Completed: p.completed.Load(),
		Panics:    p.panics.Load(),
	}
}

// worker drains the task channel until Close closes it.
func (p *Pool) worker() {
	defer p.wg.Done()
	for task := range p.tasks {
		p.runTask(task)
	}
}

// runTask isolates one task: a panic is recovered (and optionally reported)
// so it can never kill the worker or the process. Active drops before
// Completed rises, so a task is never counted as running and finished at the
// same time.
func (p *Pool) runTask(task Task) {
	p.active.Add(1)
	defer func() {
		p.active.Add(-1)
		if r := recover(); r != nil {
			p.panics.Add(1)
			if p.onPanic != nil {
				p.onPanic(r)
			}
		}
		p.completed.Add(1)
	}()
	task()
}
