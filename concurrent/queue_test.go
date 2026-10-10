package concurrent_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neko233-com/struct233-go/concurrent"
)

func TestQueueFIFO(t *testing.T) {
	q := concurrent.NewQueue[int](3)
	require.Equal(t, 3, q.Cap())
	require.Zero(t, q.Len())

	require.True(t, q.Put(1))
	require.True(t, q.Put(2))
	require.True(t, q.Put(3))
	require.Equal(t, 3, q.Len())
	require.False(t, q.TryPut(4), "TryPut must fail on a full queue")

	for _, want := range []int{1, 2, 3} {
		got, ok := q.Take()
		require.True(t, ok)
		require.Equal(t, want, got, "queue must be FIFO")
	}
	require.Zero(t, q.Len())
}

func TestQueueTryTakeAndPoll(t *testing.T) {
	q := concurrent.NewQueue[string](2)

	_, ok := q.TryTake()
	require.False(t, ok, "TryTake on empty queue must miss")

	_, ok = q.Poll(10 * time.Millisecond)
	require.False(t, ok, "Poll must time out on an empty queue")

	require.True(t, q.TryPut("a"))
	v, ok := q.Poll(0)
	require.True(t, ok)
	require.Equal(t, "a", v)

	require.True(t, q.TryPut("b"))
	v, ok = q.Poll(50 * time.Millisecond)
	require.True(t, ok)
	require.Equal(t, "b", v)
}

func TestQueueTakeBlocksUntilPut(t *testing.T) {
	q := concurrent.NewQueue[int](1)

	done := make(chan int, 1)
	go func() {
		v, ok := q.Take()
		if !ok {
			done <- -1
			return
		}
		done <- v
	}()

	time.Sleep(20 * time.Millisecond) // let the consumer block
	require.True(t, q.Put(7))

	select {
	case v := <-done:
		require.Equal(t, 7, v)
	case <-time.After(time.Second):
		t.Fatal("Take was not unblocked by Put")
	}
}

func TestQueueOfferWaitsForSpace(t *testing.T) {
	q := concurrent.NewQueue[int](1)
	require.True(t, q.TryPut(1))

	require.False(t, q.Offer(2, 0), "Offer without timeout must fail on a full queue")
	require.False(t, q.Offer(2, 20*time.Millisecond), "Offer must time out on a full queue")

	done := make(chan bool, 1)
	go func() { done <- q.Offer(3, time.Second) }()

	time.Sleep(20 * time.Millisecond)
	v, ok := q.Take()
	require.True(t, ok)
	require.Equal(t, 1, v)

	select {
	case enqueued := <-done:
		require.True(t, enqueued, "Offer must succeed once space appears")
	case <-time.After(time.Second):
		t.Fatal("Offer was not unblocked by Take")
	}

	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, 3, v)
}

func TestQueueCloseUnblocksEmptyTake(t *testing.T) {
	q := concurrent.NewQueue[int](1)

	takeDone := make(chan bool, 1)
	go func() {
		_, ok := q.Take()
		takeDone <- ok
	}()
	time.Sleep(20 * time.Millisecond) // let the consumer block on the empty queue

	q.Close()
	require.True(t, q.Closed())

	select {
	case ok := <-takeDone:
		require.False(t, ok, "Take must report false once closed and drained")
	case <-time.After(time.Second):
		t.Fatal("Take was not unblocked by Close")
	}
}

func TestQueueCloseUnblocksFullPut(t *testing.T) {
	q := concurrent.NewQueue[int](1)
	require.True(t, q.TryPut(1))
	require.False(t, q.TryPut(2), "queue is full")

	putDone := make(chan bool, 1)
	go func() { putDone <- q.Put(3) }()
	time.Sleep(20 * time.Millisecond) // let the producer block on the full queue

	q.Close()

	select {
	case ok := <-putDone:
		require.False(t, ok, "Put must report false once the queue is closed")
	case <-time.After(time.Second):
		t.Fatal("Put was not unblocked by Close")
	}
	require.False(t, q.Put(4))
	require.False(t, q.Offer(4, 0))
	require.False(t, q.Offer(4, 10*time.Millisecond))
}

func TestQueueDrainAfterClose(t *testing.T) {
	q := concurrent.NewQueue[int](3)
	require.True(t, q.TryPut(1))
	require.True(t, q.TryPut(2))

	q.Close()

	v, ok := q.Take()
	require.True(t, ok)
	require.Equal(t, 1, v)

	v, ok = q.Take()
	require.True(t, ok)
	require.Equal(t, 2, v)

	_, ok = q.Take()
	require.False(t, ok, "Take must miss once the queue is closed and drained")

	_, ok = q.Poll(10 * time.Millisecond)
	require.False(t, ok)
}

func TestQueueAll(t *testing.T) {
	q := concurrent.NewQueue[int](3)
	q.Put(1)
	q.Put(2)
	q.Put(3)

	require.Equal(t, []int{1, 2, 3}, q.All())
	require.Equal(t, 3, q.Len(), "All must not drain the queue")

	_, _ = q.Take()
	require.Equal(t, []int{2, 3}, q.All())
}

func TestQueueConcurrentProducersAndConsumers(t *testing.T) {
	const (
		producers = 4
		consumers = 4
		perWorker = 250
	)
	q := concurrent.NewQueue[int](32)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		consumed int
	)
	for range producers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				q.Put(i)
			}
		}()
	}
	for range consumers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := q.Poll(50 * time.Millisecond)
				if !ok {
					return
				}
				mu.Lock()
				consumed++
				mu.Unlock()
			}
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("producers/consumers did not finish")
	}

	// every produced element is either consumed or still queued
	require.Equal(t, producers*perWorker, consumed+q.Len())
}

func TestQueuePanicsOnBadCapacity(t *testing.T) {
	require.Panics(t, func() { concurrent.NewQueue[int](0) })
	require.Panics(t, func() { concurrent.NewQueue[int](-1) })
}

func BenchmarkQueuePutTake(b *testing.B) {
	q := concurrent.NewQueue[int](1024)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		q.Put(i)
		q.Take()
	}
}

func BenchmarkQueueTryPutTryTake(b *testing.B) {
	q := concurrent.NewQueue[int](1024)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		q.TryPut(i)
		q.TryTake()
	}
}
