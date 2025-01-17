// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package recovery

import (
	"context"

	"github.com/yanhuangpai/voyager/pkg/crypto"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/pss"
	"github.com/yanhuangpai/voyager/pkg/pushsync"
	"github.com/yanhuangpai/voyager/pkg/storage"
)

const (
	// TopicText is the string used to construct the recovery topic.
	TopicText = "RECOVERY"
)

var (
	// Topic is the topic used for repairing globally pinned chunks.
	Topic = pss.NewTopic(TopicText)
)

// Callback defines code to be executed upon failing to retrieve chunks.
type Callback func(chunkAddress infinity.Address, targets pss.Targets)

// NewsCallback returns a new Callback with the sender function defined.
func NewCallback(pssSender pss.Sender) Callback {
	privk := crypto.Secp256k1PrivateKeyFromBytes([]byte(TopicText))
	recipient := privk.PublicKey
	return func(chunkAddress infinity.Address, targets pss.Targets) {
		payload := chunkAddress
		ctx := context.Background()
		_ = pssSender.Send(ctx, Topic, payload.Bytes(), &recipient, targets)
	}
}

// NewRepairHandler creates a repair function to re-upload globally pinned chunks to the network with the given store.
func NewRepairHandler(s storage.Storer, logger logging.Logger, pushSyncer pushsync.PushSyncer) pss.Handler {
	return func(ctx context.Context, m []byte) {
		chAddr := m

		// check if the chunk exists in the local store and proceed.
		// otherwise the Get will trigger a unnecessary network retrieve
		exists, err := s.Has(ctx, infinity.NewAddress(chAddr))
		if err != nil {
			return
		}
		if !exists {
			return
		}

		// retrieve the chunk from the local store
		ch, err := s.Get(ctx, storage.ModeGetRequest, infinity.NewAddress(chAddr))
		if err != nil {
			logger.Tracef("chunk repair: error while getting chunk for repairing: %v", err)
			return
		}

		// push the chunk using push sync so that it reaches it destination in network
		_, err = pushSyncer.PushChunkToClosest(ctx, ch)
		if err != nil {
			logger.Tracef("chunk repair: error while sending chunk or receiving receipt: %v", err)
			return
		}
	}
}
