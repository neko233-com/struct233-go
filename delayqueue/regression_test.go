package delayqueue_test

import (
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/struct233-go/delayqueue"
	"github.com/stretchr/testify/require"
)

func TestRepeatedElementPreservesEachInsertionPosition(t *testing.T) {
	q := delayqueue.New[string]()
	deadline := time.Now().Add(-time.Second)
	a := delayqueue.NewElement("a", deadline)
	b := delayqueue.NewElement("b", deadline)
	head := delayqueue.NewElement("head", deadline)
	q.Put(head)
	q.Put(a)
	q.Put(b)
	q.Put(a)
	got := q.Drain(0)
	require.Len(t, got, 4)
	require.Same(t, head, got[0])
	require.Same(t, a, got[1])
	require.Same(t, b, got[2])
	require.Same(t, a, got[3])
}

func TestElementCanBeSharedAcrossConcurrentQueues(t *testing.T) {
	e := delayqueue.NewElement(42, time.Now().Add(-time.Second))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := delayqueue.New[int]()
			for range 1000 {
				q.Put(e)
				q.TryTake()
			}
		}()
	}
	wg.Wait()
}
