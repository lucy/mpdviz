package main

import (
	"encoding/binary"
	"testing"
)

type byteSliceReader struct {
	remain []byte
}

func (br *byteSliceReader) Read(p []byte) (int, error) {
	n := copy(p, br.remain)
	br.remain = br.remain[n:]
	return n, nil
}

func BenchmarkBinaryReadSlice1000Int16(b *testing.B) {
	bsr := &byteSliceReader{}
	slice := make([]int16, 1000)
	buf := make([]byte, len(slice)*4)
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bsr.remain = buf
		binary.Read(bsr, binary.LittleEndian, slice)
	}
}

func BenchmarkReadSlice1000Int16(b *testing.B) {
	bsr := &byteSliceReader{}
	slice := make([]int16, 1000)
	buf := make([]byte, len(slice)*4)
	b.SetBytes(int64(len(buf)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bsr.remain = buf
		readInt16s(bsr, slice)
	}
}
