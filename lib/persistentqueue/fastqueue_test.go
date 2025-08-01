package persistentqueue

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFastQueueOpenClose(_ *testing.T) {
	path := "fast-queue-open-close"
	fs.MustRemoveDir(path)
	for i := 0; i < 10; i++ {
		fq := MustOpenFastQueue(path, "foobar", 100, 0, false)
		fq.MustClose()
	}
	fs.MustRemoveDir(path)
}

func TestFastQueueWriteReadInmemory(t *testing.T) {
	path := "fast-queue-write-read-inmemory"
	fs.MustRemoveDir(path)

	capacity := 100
	fq := MustOpenFastQueue(path, "foobar", capacity, 0, false)
	if n := fq.GetInmemoryQueueLen(); n != 0 {
		t.Fatalf("unexpected non-zero inmemory queue size:  %d", n)
	}
	var blocks []string
	for i := 0; i < capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		if !fq.TryWriteBlock([]byte(block)) {
			t.Fatalf("TryWriteBlock must return true in this context")
		}
		blocks = append(blocks, block)
	}
	if n := fq.GetInmemoryQueueLen(); n != capacity {
		t.Fatalf("unexpected size of inmemory queue; got %d; want %d", n, capacity)
	}
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}

func TestFastQueueWriteReadMixed(t *testing.T) {
	path := "fast-queue-write-read-mixed"
	fs.MustRemoveDir(path)

	capacity := 100
	fq := MustOpenFastQueue(path, "foobar", capacity, 0, false)
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	var blocks []string
	for i := 0; i < 2*capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		if !fq.TryWriteBlock([]byte(block)) {
			t.Fatalf("TryWriteBlock must return true in this context")
		}
		blocks = append(blocks, block)
	}
	if n := fq.GetPendingBytes(); n == 0 {
		t.Fatalf("the number of pending bytes must be greater than 0")
	}
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}

func TestFastQueueWriteReadWithCloses(t *testing.T) {
	path := "fast-queue-write-read-with-closes"
	fs.MustRemoveDir(path)

	capacity := 100
	fq := MustOpenFastQueue(path, "foobar", capacity, 0, false)
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	var blocks []string
	for i := 0; i < 2*capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		if !fq.TryWriteBlock([]byte(block)) {
			t.Fatalf("TryWriteBlock must return true in this context")
		}

		blocks = append(blocks, block)
		fq.MustClose()
		fq = MustOpenFastQueue(path, "foobar", capacity, 0, false)
	}
	if n := fq.GetPendingBytes(); n == 0 {
		t.Fatalf("the number of pending bytes must be greater than 0")
	}
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
		fq.MustClose()
		fq = MustOpenFastQueue(path, "foobar", capacity, 0, false)
	}
	if n := fq.GetPendingBytes(); n != 0 {
		t.Fatalf("the number of pending bytes must be 0; got %d", n)
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}

func TestFastQueueReadUnblockByClose(t *testing.T) {
	path := "fast-queue-read-unblock-by-close"
	fs.MustRemoveDir(path)

	fq := MustOpenFastQueue(path, "foorbar", 123, 0, false)
	resultCh := make(chan error)
	go func() {
		data, ok := fq.MustReadBlock(nil)
		if ok {
			resultCh <- fmt.Errorf("unexpected ok=true")
			return
		}
		if len(data) != 0 {
			resultCh <- fmt.Errorf("unexpected non-empty data=%q", data)
			return
		}
		resultCh <- nil
	}()
	fq.MustClose()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
	fs.MustRemoveDir(path)
}

func TestFastQueueReadUnblockByWrite(t *testing.T) {
	path := "fast-queue-read-unblock-by-write"
	fs.MustRemoveDir(path)

	fq := MustOpenFastQueue(path, "foobar", 13, 0, false)
	block := "foodsafdsaf sdf"
	resultCh := make(chan error)
	go func() {
		data, ok := fq.MustReadBlock(nil)
		if !ok {
			resultCh <- fmt.Errorf("unexpected ok=false")
			return
		}
		if string(data) != block {
			resultCh <- fmt.Errorf("unexpected block read; got %q; want %q", data, block)
			return
		}
		resultCh <- nil
	}()
	if !fq.TryWriteBlock([]byte(block)) {
		t.Fatalf("TryWriteBlock must return true in this context")
	}
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout")
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}

func TestFastQueueReadWriteConcurrent(t *testing.T) {
	path := "fast-queue-read-write-concurrent"
	fs.MustRemoveDir(path)

	fq := MustOpenFastQueue(path, "foobar", 5, 0, false)

	var blocks []string
	blocksMap := make(map[string]bool)
	var blocksMapLock sync.Mutex
	for i := 0; i < 1000; i++ {
		block := fmt.Sprintf("block %d", i)
		blocks = append(blocks, block)
		blocksMap[block] = true
	}

	// Start readers
	var readersWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		readersWG.Add(1)
		go func() {
			defer readersWG.Done()
			for {
				data, ok := fq.MustReadBlock(nil)
				if !ok {
					return
				}
				blocksMapLock.Lock()
				if !blocksMap[string(data)] {
					panic(fmt.Errorf("unexpected data read from the queue: %q", data))
				}
				delete(blocksMap, string(data))
				blocksMapLock.Unlock()
			}
		}()
	}

	// Start writers
	blocksCh := make(chan string)
	var writersWG sync.WaitGroup
	for i := 0; i < 10; i++ {
		writersWG.Add(1)
		go func() {
			defer writersWG.Done()
			for block := range blocksCh {
				if !fq.TryWriteBlock([]byte(block)) {
					panic(fmt.Errorf("TryWriteBlock must return true in this context"))
				}
			}
		}()
	}

	// feed writers
	for _, block := range blocks {
		blocksCh <- block
	}
	close(blocksCh)

	// Wait for writers to finish
	writersWG.Wait()

	// wait for a while, so readers could catch up
	time.Sleep(100 * time.Millisecond)

	// Close fq
	fq.MustClose()

	// Wait for readers to finish
	readersWG.Wait()

	// Collect the remaining data
	fq = MustOpenFastQueue(path, "foobar", 5, 0, false)
	resultCh := make(chan error)
	go func() {
		for len(blocksMap) > 0 {
			data, ok := fq.MustReadBlock(nil)
			if !ok {
				resultCh <- fmt.Errorf("unexpected ok=false")
				return
			}
			if !blocksMap[string(data)] {
				resultCh <- fmt.Errorf("unexpected data read from fq: %q", data)
				return
			}
			delete(blocksMap, string(data))
		}
		resultCh <- nil
	}()
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout")
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}

func TestFastQueueWriteReadWithDisabledPQ(t *testing.T) {
	path := "fast-queue-write-read-inmemory-disabled-pq"
	fs.MustRemoveDir(path)

	capacity := 20
	fq := MustOpenFastQueue(path, "foobar", capacity, 0, true)
	if n := fq.GetInmemoryQueueLen(); n != 0 {
		t.Fatalf("unexpected non-zero inmemory queue size:  %d", n)
	}
	var blocks []string
	for i := 0; i < capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		if !fq.TryWriteBlock([]byte(block)) {
			t.Fatalf("TryWriteBlock must return true in this context")
		}
		blocks = append(blocks, block)
	}
	if fq.TryWriteBlock([]byte("error-block")) {
		t.Fatalf("expect false due to full queue")
	}

	fq.MustClose()
	fq = MustOpenFastQueue(path, "foobar", capacity, 0, true)
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}

func TestFastQueueWriteReadWithIgnoreDisabledPQ(t *testing.T) {
	path := "fast-queue-write-read-inmemory-disabled-pq-force-write"
	fs.MustRemoveDir(path)

	capacity := 20
	fq := MustOpenFastQueue(path, "foobar", capacity, 0, true)
	if n := fq.GetInmemoryQueueLen(); n != 0 {
		t.Fatalf("unexpected non-zero inmemory queue size:  %d", n)
	}
	var blocks []string
	for i := 0; i < capacity; i++ {
		block := fmt.Sprintf("block %d", i)
		if !fq.TryWriteBlock([]byte(block)) {
			t.Fatalf("TryWriteBlock must return true in this context")
		}
		blocks = append(blocks, block)
	}
	if fq.TryWriteBlock([]byte("error-block")) {
		t.Fatalf("expect false due to full queue")
	}
	for i := 0; i < capacity; i++ {
		block := fmt.Sprintf("block %d-%d", i, i)
		fq.MustWriteBlockIgnoreDisabledPQ([]byte(block))
		blocks = append(blocks, block)
	}

	fq.MustClose()
	fq = MustOpenFastQueue(path, "foobar", capacity, 0, true)
	for _, block := range blocks {
		buf, ok := fq.MustReadBlock(nil)
		if !ok {
			t.Fatalf("unexpected ok=false")
		}
		if string(buf) != block {
			t.Fatalf("unexpected block read; got %q; want %q", buf, block)
		}
	}
	fq.MustClose()
	fs.MustRemoveDir(path)
}
