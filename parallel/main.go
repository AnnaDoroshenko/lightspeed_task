package main

import (
	"bufio"
	"fmt"
	"io"
	"math/bits"
	"os"
	"sync"
	"time"
)

const (
	file          = "ip_addresses"
	chunkSize     = 1024 * 8
	lfCharCode    = 10
	maxIpLen      = 15
	minIpLen      = 7
	addrChunkSize = chunkSize / minIpLen
	workerCount   = 4
)

func main() {
	start := time.Now()

	ipAddrs := make([]uint64, 1<<26)
	addrs := make(chan Result)
	honks := make(chan int, workerCount)
	var wg sync.WaitGroup
	workerPool := [workerCount]Worker{}

	// Create and start workers
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		worker := NewWorker(w)
		workerPool[w] = *worker
		go worker.Run(addrs, &wg)
		honks <- w
	}

	go func() {
		defer close(honks)
		for i := 0; i < workerCount; i++ {
			defer workerPool[i].Close()
		}

		f, err := os.Open(file)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		reader := bufio.NewReader(f)
		start := 0
		remainder := make([]byte, chunkSize)
		// Wait until a worker is ready for a new chunk to work on
		for id := range honks {
			if start != 0 {
				for i := 0; i < start; i++ {
					workerPool[id].Buf[i] = remainder[i] // Copy the remainder
				}
			}

			// Read a new chunk from file directly into the worker's buffer
			bytesRead, err := reader.Read(workerPool[id].Buf[start:])
			if err != nil {
				if err == io.EOF {
					break
				}
				panic(err)
			}

			// Find the last newline to make sure the worker always finishes on a valid IP addr end
			length := start + bytesRead
			end := length - 1
			for workerPool[id].Buf[end] != lfCharCode {
				end--
			}
			end++ // First after the delimiter

			// Signal the worker that a new chunk has been read to its buffer
			workerPool[id].Ends <- end

			// Store the remainder for the next worker
			for start = 0; start+end < length; start++ {
				remainder[start] = workerPool[id].Buf[start+end]
			}
		}
	}()

	go func() {
		wg.Wait()
		close(addrs)
	}()

	// Wait until workers are done processing their chunks
	for result := range addrs {
		for i := 0; i < result.index; i++ {
			addr := workerPool[result.id].Result[i]
			subAddr := addr & 63
			mainAddr := addr >> 6
			ipAddrs[mainAddr] |= 1 << subAddr
		}
		// Signal that the result has been processed and the worker is ready to start the next chunk
		honks <- result.id
	}

	count := 0
	for _, v := range ipAddrs {
		count += bits.OnesCount64(v)
	}
	fmt.Printf("Result: %d unique addresses\n", count)

	elapsed := time.Since(start)
	fmt.Printf("Counting unique addresses took %s\n", elapsed)
}

type Result struct {
	id    int
	index int
}

type Worker struct {
	Id     int
	Buf    []byte
	Ends   chan int
	Result []uint32
}

func NewWorker(id int) *Worker {
	buf := make([]byte, chunkSize)
	ends := make(chan int)
	result := make([]uint32, addrChunkSize)

	return &Worker{
		Id:     id,
		Buf:    buf,
		Ends:   ends,
		Result: result,
	}
}

func (worker *Worker) Close() {
	close(worker.Ends)
}

func (worker *Worker) Run(addrs chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	// Wait until a new chunk has been read into the worker's buffer
	for end := range worker.Ends {
		start := 0
		resultIndex := 0
		for ; start != end; resultIndex++ {
			addr := worker.parseAddr(&start)
			worker.Result[resultIndex] = addr
		}
		// Signal that addresses from current chunk are ready
		addrs <- Result{worker.Id, resultIndex}
	}
}

func (worker *Worker) parseAddr(it *int) uint32 {
	var ipAddr uint32 = 0
	for i := 0; i < 4; i++ {
		var block uint32 = 0
		for '0' <= worker.Buf[*it] && worker.Buf[*it] <= '9' {
			block = block*10 + uint32(worker.Buf[(*it)]-'0')
			(*it)++
		}
		(*it)++
		ipAddr = (ipAddr << 8) | block
	}

	return ipAddr
}
