package main

import (
	"io"
	"reflect"
	"unsafe"
)

// BenchmarkBinaryReadSlice1000Int16   50000      50957 ns/op   78.50    MB/s
// BenchmarkReadSlice1000Int16         10000000   143   ns/op   27834.15 MB/s

func readInt16s(r io.Reader, data []int16) error {
	size := 2 * len(data)
	capa := 2 * cap(data)
	h := (*reflect.SliceHeader)(unsafe.Pointer(&data)).Data
	buf := *((*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: h, Len: size, Cap: capa})))
	_, err := io.ReadFull(r, buf)
	return err
}
