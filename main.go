package main

import (
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

	ipProcessor, err := newIpProcessor(file)
	if err != nil {
		panic(err)
	}
	defer ipProcessor.Close()

	count, err := ipProcessor.CountUnique()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Result: %d unique addresses\n", count)

	elapsed := time.Since(start)
	fmt.Printf("Counting unique addresses took %s\n", elapsed)
}

type ipProcessor struct {
	FileHandler *os.File
	Buf         []byte
	IpAddrs     []uint64
}

func newIpProcessor(file string) (*ipProcessor, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, chunkSize)
	ipAddrs := make([]uint64, 1<<26)

	return &ipProcessor{
		FileHandler: f,
		Buf:         buf,
		IpAddrs:     ipAddrs,
	}, nil
}

func (ipProcessor *ipProcessor) Close() {
	ipProcessor.FileHandler.Close()
}

func (ipProcessor *ipProcessor) CountUnique() (int, error) {
	start := 0
	for {
		bytesRead, err := ipProcessor.FileHandler.Read(ipProcessor.Buf[start:])
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}

		length := start + bytesRead
		if length == 0 {
			break
		}

		end := length - 1
		for ipProcessor.Buf[end] != lfCharCode {
			end--
		}
		end++ // first after delimiter

		start = 0
		for start != end {
			addr := ipProcessor.parseAddr(&start)
			ipProcessor.remember(&addr)
		}

		for start = 0; start+end < length; start++ {
			ipProcessor.Buf[start] = ipProcessor.Buf[start+end]
		}
	}

	count := ipProcessor.onesCount()

	return count, nil
}

func (ipProcessor *ipProcessor) parseAddr(it *int) uint32 {
	var ipAddr uint32 = 0
	for i := 0; i < 4; i++ {
		var block uint32 = 0
		for '0' <= ipProcessor.Buf[*it] && ipProcessor.Buf[*it] <= '9' {
			block = block*10 + uint32(ipProcessor.Buf[(*it)]-'0')
			(*it)++
		}
		(*it)++
		ipAddr = (ipAddr << 8) | block
	}

	return ipAddr
}

func (ipProcessor *ipProcessor) remember(addr *uint32) {
	subAddr := *addr & 63
	mainAddr := *addr >> 6
	ipProcessor.IpAddrs[mainAddr] |= 1 << subAddr
}

func (ipProcessor *ipProcessor) onesCount() int {
	count := 0
	for _, v := range ipProcessor.IpAddrs {
		count += bits.OnesCount64(v)
	}

	return count
}
