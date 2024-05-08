package cac

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/yanhuangpai/voyager/pkg/bmtpool"
	"github.com/yanhuangpai/voyager/pkg/infinity"
)

var (
	errTooShortChunkData = errors.New("short chunk data")
	errTooLargeChunkData = errors.New("data too large")
)

// New creates a new content address chunk by initializing a span and appending the data to it.
func New(data []byte) (infinity.Chunk, error) {
	dataLength := len(data)
	if dataLength > infinity.ChunkSize {
		return nil, errTooLargeChunkData
	}

	if dataLength == 0 {
		return nil, errTooShortChunkData
	}

	span := make([]byte, infinity.SpanSize)
	binary.LittleEndian.PutUint64(span, uint64(dataLength))
	return newWithSpan(data, span)
}

// NewWithDataSpan creates a new chunk assuming that the span precedes the actual data.
func NewWithDataSpan(data []byte) (infinity.Chunk, error) {
	dataLength := len(data)
	if dataLength > infinity.ChunkSize+infinity.SpanSize {
		return nil, errTooLargeChunkData
	}

	if dataLength < infinity.SpanSize {
		return nil, errTooShortChunkData
	}
	return newWithSpan(data[infinity.SpanSize:], data[:infinity.SpanSize])
}

// newWithSpan creates a new chunk prepending the given span to the data.
func newWithSpan(data, span []byte) (infinity.Chunk, error) {
	h := hasher(data)
	hash, err := h(span)
	if err != nil {
		return nil, err
	}

	cdata := make([]byte, len(data)+len(span))
	copy(cdata[:infinity.SpanSize], span)
	copy(cdata[infinity.SpanSize:], data)
	return infinity.NewChunk(infinity.NewAddress(hash), cdata), nil
}

// hasher is a helper function to hash a given data based on the given span.
func hasher(data []byte) func([]byte) ([]byte, error) {
	return func(span []byte) ([]byte, error) {
		hasher := bmtpool.Get()
		defer bmtpool.Put(hasher)

		if err := hasher.SetSpanBytes(span); err != nil {
			return nil, err
		}
		if _, err := hasher.Write(data); err != nil {
			return nil, err
		}
		return hasher.Sum(nil), nil
	}
}

// Valid checks whether the given chunk is a valid content-addressed chunk.
func Valid(c infinity.Chunk) bool {
	data := c.Data()
	if len(data) < infinity.SpanSize {
		return false
	}

	if len(data) > infinity.ChunkSize+infinity.SpanSize {
		return false
	}

	h := hasher(data[infinity.SpanSize:])
	hash, _ := h(data[:infinity.SpanSize])
	return bytes.Equal(hash, c.Address().Bytes())
}
