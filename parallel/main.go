package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/bits"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	file        = "ip_addresses"
	chunkSize   = 1024 * 8
	lfCharCode  = 10
	maxIpLen    = 15
	workerCount = 8
	bufSize     = workerCount
)

func main() {
	runtime.GOMAXPROCS(workerCount)

	start := time.Now()

	ipAddrs := make([]uint64, 1<<26)
	chunks := make(chan []byte, bufSize)
	addrs := make(chan uint32, bufSize)
	var wg sync.WaitGroup

	// Fan-out: Start workers
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go worker(chunks, addrs, &wg)
	}

	// Send chunks to the chunks channel
	go func() {
		defer close(chunks)

		f, err := os.Open(file)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		reader := bufio.NewReader(f)
		buf := make([]byte, chunkSize)
		lastTerminator := 0
		for {
			bytesRead, err := reader.Read(buf[lastTerminator:])
			if err != nil {
				if err == io.EOF {
					break
				}
				panic(err)
			}

			bufLen := lastTerminator + bytesRead
			lastTerminator = bytes.LastIndexByte(buf[:bufLen], lfCharCode)

			bufCopy := make([]byte, len(buf[:lastTerminator+1]))
			copy(bufCopy, buf[:lastTerminator+1])
			chunks <- bufCopy

			remainingLen := bufLen - lastTerminator - 1
			for i := 0; i < remainingLen; i++ {
				buf[i] = buf[i+lastTerminator+1]
			}
			lastTerminator = remainingLen
		}
	}()

	// Fan-in: Close addrs channel once all workers complete
	go func() {
		wg.Wait()
		close(addrs)
	}()

	for addr := range addrs {
		subAddr := addr & 63
		mainAddr := addr >> 6
		ipAddrs[mainAddr] |= 1 << subAddr
	}

	count := 0
	for _, v := range ipAddrs {
		count += bits.OnesCount64(v)
	}
	fmt.Printf("Result: %d unique addresses\n", count)

	elapsed := time.Since(start)
	fmt.Printf("Counting unique addresses took %s\n", elapsed)
}

// Worker processes chunks and sends results to the addrs channel
func worker(chunks <-chan []byte, addrs chan<- uint32, wg *sync.WaitGroup) {
	defer wg.Done()

	tmp := make([]byte, maxIpLen)
	tmpLen := 0
	for chunk := range chunks {
		for _, char := range chunk {
			if char == lfCharCode || len(tmp) == tmpLen {
				curAddr, ok := readIpAddr(tmp, tmpLen)
				if !ok {
					return
				}
				addrs <- curAddr
				tmpLen = 0
			} else {
				tmp[tmpLen] = char
				tmpLen++
			}
		}
	}
}

func readBlock(buf []byte, index int, bufLen int) (int, uint8, bool) {
	const dotCharCode = 46

	var block uint8 = 0
	readCount := 0
	for {
		readCount++
		if index == bufLen || buf[index] == dotCharCode {
			return readCount, block, true
		}
		block = block*10 + uint8(buf[index]-'0')
		index++
	}
}

func readIpAddr(buf []byte, bufLen int) (uint32, bool) {
	var ipAddr uint32 = 0
	var offset uint32 = 24 // 3 bytes

	for index := 0; index < bufLen; {
		readCount, block, ok := readBlock(buf, index, bufLen)
		if !ok {
			return 0, false
		}

		ipAddr |= uint32(block) << offset
		offset -= 8
		index += readCount
	}

	return ipAddr, true
}
