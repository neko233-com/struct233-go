package workerpool_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/workerpool"
)

func TestCloseDrainsPendingTasks(t *testing.T) {
	const total = 200
	p := workerpool.New(4, workerpool.WithQueueSize(total))

	var counter atomic.Int64
	for range total {
		require.NoError(t, p.Submit(func() { counter.Add(1) }))
	}
	p.Close()

	require.Equal(t, int64(total), counter.Load(), "Close must let every queued task run")
	require.Equal(t, int64(total), p.Metrics().Completed)
	require.Equal(t, int64(total), p.Metrics().Submitted)
	require.Zero(t, p.Metrics().Active, "no worker may be running once Close returns")
	require.Zero(t, p.Metrics().Queued, "no task may be left queued once Close returns")
}

func TestSubmitAfterCloseReportsErrClosed(t *testing.T) {
	p := workerpool.New(1)
	p.Close()

	require.ErrorIs(t, p.Submit(func() {}), workerpool.ErrClosed)
}

func TestTrySubmitAfterCloseReportsErrClosed(t *testing.T) {
	p := workerpool.New(1)
	p.Close()

	require.ErrorIs(t, p.TrySubmit(func() {}), workerpool.ErrClosed)
}

func TestSubmitWaitAfterCloseReportsErrClosed(t *testing.T) {
	p := workerpool.New(1)
	p.Close()

	require.ErrorIs(t, p.SubmitWait(func() {}), workerpool.ErrClosed)
}

func TestSubmitWaitRunsTaskBeforeReturning(t *testing.T) {
	p := workerpool.New(1)
	defer p.Close()

	var order []string
	require.NoError(t, p.SubmitWait(func() { order = append(order, "task") }))
	order = append(order, "after")

	require.Equal(t, []string{"task", "after"}, order,
		"the task must have run before SubmitWait returns")
}

func TestTrySubmitReportsFullQueue(t *testing.T) {
	p := workerpool.New(1, workerpool.WithQueueSize(1))
	defer p.Close()

	entered := make(chan struct{})
	release := make(chan struct{})
	require.NoError(t, p.Submit(func() { close(entered); <-release }))
	<-entered // the only worker is now busy

	require.NoError(t, p.Submit(func() {}), "the free queue slot must accept one task")
	require.ErrorIs(t, p.TrySubmit(func() {}), workerpool.ErrQueueFull,
		"TrySubmit must not queue while the worker is busy and the queue is full")

	close(release)
}

func TestSubmitBlocksWhileQueueIsFull(t *testing.T) {
	p := workerpool.New(1, workerpool.WithQueueSize(1))
	defer p.Close()

	entered := make(chan struct{})
	release := make(chan struct{})
	require.NoError(t, p.Submit(func() { close(entered); <-release }))
	<-entered                               // the only worker is now busy
	require.NoError(t, p.Submit(func() {})) // fills the single queue slot

	blocked := make(chan error, 1)
	go func() { blocked <- p.Submit(func() {}) }()

	select {
	case err := <-blocked:
		require.NoError(t, err)
		t.Fatal("Submit must block while the queue is full instead of dropping the task")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-blocked:
		require.NoError(t, err, "Submit must succeed once a worker frees a slot")
	case <-time.After(time.Second):
		t.Fatal("Submit was not unblocked when the worker drained the queue")
	}

	p.Close()
	require.Equal(t, int64(3), p.Metrics().Completed, "no accepted task may be lost")
}

func TestPanicDoesNotKillWorker(t *testing.T) {
	recovered := make(chan any, 1)
	p := workerpool.New(1, workerpool.WithPanicHandler(func(r any) { recovered <- r }))
	defer p.Close()

	require.NoError(t, p.Submit(func() { panic("boom") }))

	select {
	case r := <-recovered:
		require.Equal(t, "boom", r, "the handler must receive the recovered value")
	case <-time.After(time.Second):
		t.Fatal("the panic handler was not called")
	}
	require.Equal(t, int64(1), p.Metrics().Panics)

	// the worker survived the panic and keeps running tasks
	require.NoError(t, p.SubmitWait(func() {}))
	require.Eventually(t, func() bool { return p.Metrics().Completed == 2 },
		time.Second, 10*time.Millisecond, "the worker must still complete tasks after a panic")
	require.Equal(t, int64(1), p.Metrics().Panics, "only the panicking task counts as a panic")
}

func TestPanicWithoutHandlerIsCounted(t *testing.T) {
	// a nil handler must behave like the default: count the panic, stay alive
	p := workerpool.New(1, workerpool.WithPanicHandler(nil))
	defer p.Close()

	require.NoError(t, p.Submit(func() { panic("silent") }))
	require.Eventually(t, func() bool { return p.Metrics().Panics == 1 },
		time.Second, 10*time.Millisecond, "panics must be counted without a handler")

	require.NoError(t, p.SubmitWait(func() {}))
	require.Eventually(t, func() bool { return p.Metrics().Completed == 2 },
		time.Second, 10*time.Millisecond, "a handler-less panic must not kill the worker")
}

func TestMetricsSnapshot(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	p := workerpool.New(1, workerpool.WithQueueSize(4))
	defer p.Close()

	require.NoError(t, p.Submit(func() { close(entered); <-release }))
	<-entered
	require.NoError(t, p.Submit(func() {})) // waits in the queue: the worker is busy

	busy := p.Metrics()
	require.Equal(t, 1, busy.Workers)
	require.Equal(t, 1, busy.Queued)
	require.Equal(t, 4, busy.QueueCap)
	require.Equal(t, 1, busy.Active, "the busy worker must count as active")
	require.Equal(t, int64(2), busy.Submitted)
	require.Zero(t, busy.Completed)
	require.Zero(t, busy.Panics)

	close(release)
	p.Close()

	idle := p.Metrics()
	require.Zero(t, idle.Active, "Active must drop back to zero once the pool is idle")
	require.Zero(t, idle.Queued)
	require.Equal(t, int64(2), idle.Completed)
	require.Equal(t, int64(2), idle.Submitted)
}

func TestQueueSizeDefaultsTo1024(t *testing.T) {
	p := workerpool.New(2)
	defer p.Close()

	require.Equal(t, 1024, p.Metrics().QueueCap)
	require.Equal(t, 2, p.Metrics().Workers)
}

func TestWithQueueSizeOverridesCapacity(t *testing.T) {
	p := workerpool.New(1, workerpool.WithQueueSize(8))
	defer p.Close()

	require.Equal(t, 8, p.Metrics().QueueCap)
}

func TestUnbufferedQueueHandsOffDirectly(t *testing.T) {
	p := workerpool.New(1, workerpool.WithQueueSize(0))
	defer p.Close()
	require.Zero(t, p.Metrics().QueueCap, "0 selects an unbuffered hand-off")

	entered := make(chan struct{})
	release := make(chan struct{})
	require.NoError(t, p.Submit(func() { close(entered); <-release }))
	<-entered

	// with no queue slot the task can only be handed over to a free worker
	handedOff := make(chan error, 1)
	go func() { handedOff <- p.Submit(func() {}) }()

	select {
	case err := <-handedOff:
		require.NoError(t, err)
		t.Fatal("Submit must block until a worker picks the task up on an unbuffered queue")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-handedOff:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Submit was not handed off once the worker became free")
	}
}

func TestClosedReportsState(t *testing.T) {
	p := workerpool.New(1)
	require.False(t, p.Closed(), "a fresh pool must not report closed")

	p.Close()
	require.True(t, p.Closed())
}

func TestCloseIsIdempotent(t *testing.T) {
	p := workerpool.New(4)
	for range 20 {
		require.NoError(t, p.Submit(func() {}))
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Close()
		}()
	}
	wg.Wait()

	require.True(t, p.Closed())
	require.Equal(t, int64(20), p.Metrics().Completed, "concurrent Close must still drain the queue")
}

func TestConcurrentSubmitAndClose(t *testing.T) {
	const (
		producers   = 8
		perProducer = 200
	)
	p := workerpool.New(4, workerpool.WithQueueSize(16))

	var (
		wg      sync.WaitGroup
		counter atomic.Int64
	)
	errs := make([]error, producers)
	for g := range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perProducer {
				if err := p.Submit(func() { counter.Add(1) }); err != nil {
					errs[g] = err
					return
				}
			}
		}()
	}

	time.Sleep(2 * time.Millisecond) // let the producers contend with the queue
	p.Close()
	wg.Wait()

	for g, err := range errs {
		if err != nil {
			require.ErrorIs(t, err, workerpool.ErrClosed, "producer %d saw an unexpected error", g)
		}
	}

	metrics := p.Metrics()
	require.Equal(t, metrics.Submitted, counter.Load(), "every accepted task must run")
	require.Equal(t, metrics.Submitted, metrics.Completed)
	require.Zero(t, metrics.Active)
}

func TestNoWorkerGoroutineLeaksAfterClose(t *testing.T) {
	// let goroutines started by earlier tests (testify's Eventually helper runs
	// the condition in its own goroutine) wind down before sampling
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	p := workerpool.New(8, workerpool.WithQueueSize(64))
	require.GreaterOrEqual(t, runtime.NumGoroutine(), baseline+8,
		"New must start one goroutine per worker")

	for range 100 {
		require.NoError(t, p.Submit(func() {}))
	}
	p.Close()

	// the count must fall back on its own: Close already waited for the workers
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	require.LessOrEqual(t, runtime.NumGoroutine(), baseline,
		"every worker goroutine must exit once Close returns")
}

func TestPanicsOnInvalidArguments(t *testing.T) {
	require.Panics(t, func() { workerpool.New(0) })
	require.Panics(t, func() { workerpool.New(-1) })
	require.Panics(t, func() { workerpool.New(1, workerpool.WithQueueSize(-1)) })
}
