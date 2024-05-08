// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"encoding/binary"

	"github.com/yanhuangpai/voyager/pkg/encryption"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"golang.org/x/crypto/sha3"
)

type decryptingStore struct {
	storage.Getter
}

func New(s storage.Getter) storage.Getter {
	return &decryptingStore{s}
}

func (s *decryptingStore) Get(ctx context.Context, mode storage.ModeGet, addr infinity.Address) (ch infinity.Chunk, err error) {
	switch l := len(addr.Bytes()); l {
	case infinity.HashSize:
		// normal, unencrypted content
		return s.Getter.Get(ctx, mode, addr)

	case encryption.ReferenceSize:
		// encrypted reference
		ref := addr.Bytes()
		address := infinity.NewAddress(ref[:infinity.HashSize])
		ch, err := s.Getter.Get(ctx, mode, address)
		if err != nil {
			return nil, err
		}

		d, err := decryptChunkData(ch.Data(), ref[infinity.HashSize:])
		if err != nil {
			return nil, err
		}
		return infinity.NewChunk(address, d), nil

	default:
		return nil, storage.ErrReferenceLength
	}
}

func decryptChunkData(chunkData []byte, encryptionKey encryption.Key) ([]byte, error) {
	decryptedSpan, decryptedData, err := decrypt(chunkData, encryptionKey)
	if err != nil {
		return nil, err
	}

	// removing extra bytes which were just added for padding
	length := binary.LittleEndian.Uint64(decryptedSpan)
	refSize := int64(infinity.HashSize + encryption.KeyLength)
	for length > infinity.ChunkSize {
		length = length + (infinity.ChunkSize - 1)
		length = length / infinity.ChunkSize
		length *= uint64(refSize)
	}

	c := make([]byte, length+8)
	copy(c[:8], decryptedSpan)
	copy(c[8:], decryptedData[:length])

	return c, nil
}

func decrypt(chunkData []byte, key encryption.Key) ([]byte, []byte, error) {
	decryptedSpan, err := newSpanEncryption(key).Decrypt(chunkData[:infinity.SpanSize])
	if err != nil {
		return nil, nil, err
	}
	decryptedData, err := newDataEncryption(key).Decrypt(chunkData[infinity.SpanSize:])
	if err != nil {
		return nil, nil, err
	}
	return decryptedSpan, decryptedData, nil
}

func newSpanEncryption(key encryption.Key) encryption.Interface {
	refSize := int64(infinity.HashSize + encryption.KeyLength)
	return encryption.New(key, 0, uint32(infinity.ChunkSize/refSize), sha3.NewLegacyKeccak256)
}

func newDataEncryption(key encryption.Key) encryption.Interface {
	return encryption.New(key, int(infinity.ChunkSize), 0, sha3.NewLegacyKeccak256)
}
