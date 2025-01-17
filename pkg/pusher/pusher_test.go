// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pusher_test

import (
	"context"
	"errors"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	statestore "github.com/yanhuangpai/voyager/pkg/statestore/mock"

	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/localstore"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/pusher"
	"github.com/yanhuangpai/voyager/pkg/pushsync"
	pushsyncmock "github.com/yanhuangpai/voyager/pkg/pushsync/mock"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/tags"
	"github.com/yanhuangpai/voyager/pkg/topology/mock"
)

// no of times to retry to see if we have received response from pushsync
var noOfRetries = 20

// Wrap the actual storer to intercept the modeSet that the pusher will call when a valid receipt is received
type Store struct {
	storage.Storer
	internalStorer storage.Storer
	modeSet        map[string]storage.ModeSet
	modeSetMu      *sync.Mutex

	closed              bool
	setBeforeCloseCount int
	setAfterCloseCount  int
}

// Override the Set function to capture the ModeSetSync
func (s *Store) Set(ctx context.Context, mode storage.ModeSet, addrs ...infinity.Address) error {
	s.modeSetMu.Lock()
	defer s.modeSetMu.Unlock()

	if s.closed {
		s.setAfterCloseCount++
	} else {
		s.setBeforeCloseCount++
	}

	for _, addr := range addrs {
		s.modeSet[addr.String()] = mode
	}
	return nil
}

func (s *Store) Close() error {
	s.modeSetMu.Lock()
	defer s.modeSetMu.Unlock()

	s.closed = true
	return s.internalStorer.Close()
}

// TestSendChunkToPushSync sends a chunk to pushsync to be sent ot its closest peer and get a receipt.
// once the receipt is got this check to see if the localstore is updated to see if the chunk is set
// as ModeSetSync status.
func TestSendChunkToSyncWithTag(t *testing.T) {
	// create a trigger  and a closestpeer
	triggerPeer := infinity.MustParseHexAddress("6000000000000000000000000000000000000000000000000000000000000000")
	closestPeer := infinity.MustParseHexAddress("f000000000000000000000000000000000000000000000000000000000000000")

	pushSyncService := pushsyncmock.New(func(ctx context.Context, chunk infinity.Chunk) (*pushsync.Receipt, error) {
		receipt := &pushsync.Receipt{
			Address: infinity.NewAddress(chunk.Address().Bytes()),
		}
		return receipt, nil
	})
	mtags, p, storer := createPusher(t, triggerPeer, pushSyncService, mock.WithClosestPeer(closestPeer))
	defer storer.Close()

	ta, err := mtags.Create(1)
	if err != nil {
		t.Fatal(err)
	}

	chunk := createChunk().WithTagID(ta.Uid)

	_, err = storer.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	// Check is the chunk is set as synced in the DB.
	for i := 0; i < noOfRetries; i++ {
		// Give some time for chunk to be pushed and receipt to be received
		time.Sleep(10 * time.Millisecond)

		err = checkIfModeSet(chunk.Address(), storage.ModeSetSync, storer)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}

	if ta.Get(tags.StateSynced) != 1 {
		t.Fatalf("tags error")
	}

	p.Close()
}

// TestSendChunkToPushSyncWithoutTag is similar to TestSendChunkToPushSync, excep that the tags are not
// present to simulate ifi api withotu splitter condition
func TestSendChunkToPushSyncWithoutTag(t *testing.T) {
	chunk := createChunk()

	// create a trigger  and a closestpeer
	triggerPeer := infinity.MustParseHexAddress("6000000000000000000000000000000000000000000000000000000000000000")
	closestPeer := infinity.MustParseHexAddress("f000000000000000000000000000000000000000000000000000000000000000")

	pushSyncService := pushsyncmock.New(func(ctx context.Context, chunk infinity.Chunk) (*pushsync.Receipt, error) {
		receipt := &pushsync.Receipt{
			Address: infinity.NewAddress(chunk.Address().Bytes()),
		}
		return receipt, nil
	})

	_, p, storer := createPusher(t, triggerPeer, pushSyncService, mock.WithClosestPeer(closestPeer))
	defer storer.Close()

	_, err := storer.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	// Check is the chunk is set as synced in the DB.
	for i := 0; i < noOfRetries; i++ {
		// Give some time for chunk to be pushed and receipt to be received
		time.Sleep(10 * time.Millisecond)

		err = checkIfModeSet(chunk.Address(), storage.ModeSetSync, storer)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
}

// TestSendChunkAndReceiveInvalidReceipt sends a chunk to pushsync to be sent ot its closest peer and
// get a invalid receipt (not with the address of the chunk sent). The test makes sure that this error
// is received and the ModeSetSync is not set for the chunk.
func TestSendChunkAndReceiveInvalidReceipt(t *testing.T) {
	chunk := createChunk()

	// create a trigger  and a closestpeer
	triggerPeer := infinity.MustParseHexAddress("6000000000000000000000000000000000000000000000000000000000000000")
	closestPeer := infinity.MustParseHexAddress("f000000000000000000000000000000000000000000000000000000000000000")

	pushSyncService := pushsyncmock.New(func(ctx context.Context, chunk infinity.Chunk) (*pushsync.Receipt, error) {
		return nil, errors.New("invalid receipt")
	})

	_, p, storer := createPusher(t, triggerPeer, pushSyncService, mock.WithClosestPeer(closestPeer))
	defer storer.Close()

	_, err := storer.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	// Check is the chunk is set as synced in the DB.
	for i := 0; i < noOfRetries; i++ {
		// Give some time for chunk to be pushed and receipt to be received
		time.Sleep(10 * time.Millisecond)

		err = checkIfModeSet(chunk.Address(), storage.ModeSetSync, storer)
		if err != nil {
			continue
		}
	}
	if err == nil {
		t.Fatalf("chunk not syned error expected")
	}
	p.Close()
}

// TestSendChunkAndTimeoutinReceivingReceipt sends a chunk to pushsync to be sent ot its closest peer and
// expects a timeout to get instead of getting a receipt. The test makes sure that timeout error
// is received and the ModeSetSync is not set for the chunk.
func TestSendChunkAndTimeoutinReceivingReceipt(t *testing.T) {
	chunk := createChunk()

	// create a trigger  and a closestpeer
	triggerPeer := infinity.MustParseHexAddress("6000000000000000000000000000000000000000000000000000000000000000")
	closestPeer := infinity.MustParseHexAddress("f000000000000000000000000000000000000000000000000000000000000000")

	pushSyncService := pushsyncmock.New(func(ctx context.Context, chunk infinity.Chunk) (*pushsync.Receipt, error) {
		// Set 10 times more than the time we wait for the test to complete so that
		// the response never reaches our testcase
		time.Sleep(1 * time.Second)
		return nil, nil
	})

	_, p, storer := createPusher(t, triggerPeer, pushSyncService, mock.WithClosestPeer(closestPeer))
	defer storer.Close()
	defer p.Close()

	_, err := storer.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	// Check is the chunk is set as synced in the DB.
	for i := 0; i < noOfRetries; i++ {
		// Give some time for chunk to be pushed and receipt to be received
		time.Sleep(10 * time.Millisecond)

		err = checkIfModeSet(chunk.Address(), storage.ModeSetSync, storer)
		if err != nil {
			continue
		}
	}
	if err == nil {
		t.Fatalf("chunk not syned error expected")
	}
}

func TestPusherClose(t *testing.T) {
	// create a trigger  and a closestpeer
	triggerPeer := infinity.MustParseHexAddress("6000000000000000000000000000000000000000000000000000000000000000")
	closestPeer := infinity.MustParseHexAddress("f000000000000000000000000000000000000000000000000000000000000000")

	var (
		goFuncStartedC    = make(chan struct{})
		pusherClosedC     = make(chan struct{})
		goFuncAfterCloseC = make(chan struct{})
	)

	defer func() {
		close(goFuncStartedC)
		close(pusherClosedC)
		close(goFuncAfterCloseC)
	}()

	pushSyncService := pushsyncmock.New(func(ctx context.Context, chunk infinity.Chunk) (*pushsync.Receipt, error) {
		goFuncStartedC <- struct{}{}
		<-goFuncAfterCloseC
		return nil, nil
	})

	_, p, storer := createPusher(t, triggerPeer, pushSyncService, mock.WithClosestPeer(closestPeer))

	chunk := createChunk()

	_, err := storer.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	storer.modeSetMu.Lock()
	if storer.closed == true {
		t.Fatal("store should not be closed")
	}
	if storer.setBeforeCloseCount != 0 {
		t.Fatalf("store 'Set' called %d times before close, expected 0", storer.setBeforeCloseCount)
	}
	if storer.setAfterCloseCount != 0 {
		t.Fatalf("store 'Set' called %d times after close, expected 0", storer.setAfterCloseCount)
	}
	storer.modeSetMu.Unlock()

	select {
	case <-goFuncStartedC:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting to start worker job")
	}

	// close in the background
	go func() {
		p.Close()
		storer.Close()
		pusherClosedC <- struct{}{}
	}()

	select {
	case <-pusherClosedC:
	case <-time.After(2 * time.Second):
		// internal 5 second timeout that waits for all pending push operations to terminate
	}

	storer.modeSetMu.Lock()
	if storer.setBeforeCloseCount != 0 {
		t.Fatalf("store 'Set' called %d times before close, expected 0", storer.setBeforeCloseCount)
	}
	if storer.setAfterCloseCount != 0 {
		t.Fatalf("store 'Set' called %d times after close, expected 0", storer.setAfterCloseCount)
	}
	storer.modeSetMu.Unlock()

	select {
	case goFuncAfterCloseC <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for chunk")
	}

	// we need this to allow some goroutines to complete
	time.Sleep(100 * time.Millisecond)

	storer.modeSetMu.Lock()
	if storer.closed != true {
		t.Fatal("store should be closed")
	}
	if storer.setBeforeCloseCount != 1 {
		t.Fatalf("store 'Set' called %d times before close, expected 1", storer.setBeforeCloseCount)
	}
	if storer.setAfterCloseCount != 0 {
		t.Fatalf("store 'Set' called %d times after close, expected 0", storer.setAfterCloseCount)
	}
	storer.modeSetMu.Unlock()

	// should be closed by now
	select {
	case <-pusherClosedC:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting to close pusher")
	}
}

func createChunk() infinity.Chunk {
	// chunk data to upload
	chunkAddress := infinity.MustParseHexAddress("7000000000000000000000000000000000000000000000000000000000000000")
	chunkData := []byte("1234")
	return infinity.NewChunk(chunkAddress, chunkData).WithTagID(666)
}

func createPusher(t *testing.T, addr infinity.Address, pushSyncService pushsync.PushSyncer, mockOpts ...mock.Option) (*tags.Tags, *pusher.Service, *Store) {
	t.Helper()
	logger := logging.New(ioutil.Discard, 0)
	storer, err := localstore.New("", addr.Bytes(), nil, logger)
	if err != nil {
		t.Fatal(err)
	}

	mockStatestore := statestore.NewStateStore()
	mtags := tags.NewTags(mockStatestore, logger)
	pusherStorer := &Store{
		Storer:         storer,
		internalStorer: storer,
		modeSet:        make(map[string]storage.ModeSet),
		modeSetMu:      &sync.Mutex{},
	}
	peerSuggester := mock.NewTopologyDriver(mockOpts...)

	pusherService := pusher.New(pusherStorer, peerSuggester, pushSyncService, mtags, logger, nil)
	return mtags, pusherService, pusherStorer
}

func checkIfModeSet(addr infinity.Address, mode storage.ModeSet, storer *Store) error {
	var found bool
	storer.modeSetMu.Lock()
	defer storer.modeSetMu.Unlock()

	for k, v := range storer.modeSet {
		if addr.String() == k {
			found = true
			if v != mode {
				return errors.New("chunk mode is not properly set as synced")
			}
		}
	}
	if !found {
		return errors.New("Chunk not synced")
	}
	return nil
}

// To avoid timeout during race testing
// cd pkg/pusher
// go test -race -count 1000 -timeout 60m .
