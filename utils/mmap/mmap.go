package mmap

import (
	"syscall"
	"unsafe"
)

type Mmap struct {
	data []byte
	size int
}

func New(size int) (*Mmap, error) {
	data, err := syscall.Mmap(-1, 0, size, syscall.PROT_WRITE|syscall.PROT_READ, syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS)
	if err != nil {
		return nil, err
	}
	return &Mmap{data: data, size: size}, nil
}

func (m *Mmap) ToByteSlice() []byte {
	return m.data
}

func (m *Mmap) ToUint64Slice() []uint64 {
	return unsafe.Slice((*uint64)(unsafe.Pointer(unsafe.SliceData(m.data))), m.size/8)
}

func (m *Mmap) ToFloat32Slice() []float32 {
	return unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(m.data))), m.size/4)
}

func (m *Mmap) Close() error {
	return syscall.Munmap(m.data)
}
