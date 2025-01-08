Calculate the number of unique addresses in the file using as little memory and time as possible.

Original task: https://github.com/Ecwid/new-job/blob/master/IP-Addr-Counter-GO.md

I tried a few approaches:
1. Single thread using a hashset.
2. Single thread using a packed array.
3. Multiple threads with channels.

Approach #2 turned out to be the quickest one and used the least amount of memory.

### Quick description of #1
Each IP address can be concisely represented as four 8-bit blocks, forming an int32.
Strings of IP addresses are parsed into int32s and placed into the hashset.
This solution quickly consumes too much memory due to inefficient hashset utilization.

### Quick description of #2
Same as #1, but a plain array is used to efficiently store information about whether each IP address is present.
The array is 2^26 of 2^6 (int64) cells, indexed by the high 2^26 bits of the IP address, while the low 2^6 bits are bit shifts of this IP address within the cell.
This approach is superior because the theoretical minimum amount of memory is used, and addressing takes only a few assembly instructions.

### Quick description of #3
A fan-out fan-in pattern with goroutines and channels was used, where chunks of the input file are spread into multiple workers, each parsing IP addresses and sending them into the output channel, which collects them and inserts them into the array in approach #2.
This multi-threaded approach was faster than the straight-up approach #2 because file chunks were processed concurrently. Also, we consume less memory and avoid excessive memory copying by reading from the file directly into the workers' buffers and reusing the result buffers.
