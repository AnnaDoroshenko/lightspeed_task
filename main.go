package main

import (
	"bufio"
	"fmt"
	"io"
	"math/bits"
	"os"
	"time"
)

const (
	file        = "ip_addresses"
	maxIpLen    = 15
	chunkSize   = 4096
	dotCharCode = 46
	lfCharCode  = 10
)

func main() {
	start := time.Now()

	ipAddrs := make([]uint64, 1<<26)

	ipProcessor, err := newIpProcessor(file)
	if err != nil {
		panic(err)
	}
	defer ipProcessor.Close()

	for {
		ipAddr, exists := ipProcessor.next()
		if !exists {
			break
		}
		subAddr := ipAddr & 63
		mainAddr := ipAddr >> 6
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

type ipProcessor struct {
	FileHandler *os.File
	Reader      *bufio.Reader
	Buf         []byte
	BufIndex    int
	BufLen      int
	Finished    bool
}

func newIpProcessor(file string) (*ipProcessor, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(f)
	buf := make([]byte, chunkSize)

	return &ipProcessor{
		FileHandler: f,
		Reader:      reader,
		Buf:         buf,
		BufIndex:    0,
		BufLen:      0,
		Finished:    false,
	}, nil
}

func (ipProcessor *ipProcessor) Close() {
	ipProcessor.FileHandler.Close()
}

func (ipProcessor *ipProcessor) next() (uint32, bool) {
	if ipProcessor.Finished {
		return 0, false
	}

	tmp := make([]byte, maxIpLen)
	tmpLen := 0

	for {
		if ipProcessor.BufIndex == ipProcessor.BufLen {
			var err error
			ipProcessor.BufLen, err = ipProcessor.Reader.Read(ipProcessor.Buf)
			ipProcessor.BufIndex = 0
			if err != nil {
				if err == io.EOF {
					ipProcessor.Finished = true
					if tmpLen > 0 {
						break // need to process the last IP
					}
				}
				return 0, false
			}
		}
		cur := ipProcessor.Buf[ipProcessor.BufIndex]
		ipProcessor.BufIndex++
		if cur == lfCharCode {
			break
		}
		tmp[tmpLen] = cur
		tmpLen++
	}

	return readIpAddr(tmp, tmpLen)
}

func readBlock(buf []byte, index int, bufLen int) (int, uint8, bool) {
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
