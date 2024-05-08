// Copyright 2021 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/yanhuangpai/voyager/pkg/api"
	"github.com/yanhuangpai/voyager/pkg/feeds"
	"github.com/yanhuangpai/voyager/pkg/file/loadsave"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp/jsonhttptest"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/manifest"
	testingsoc "github.com/yanhuangpai/voyager/pkg/soc/testing"
	statestore "github.com/yanhuangpai/voyager/pkg/statestore/mock"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/storage/mock"
	"github.com/yanhuangpai/voyager/pkg/tags"
)

const ownerString = "8d3766440f0d7b949a5e32995d09619a7f86e632"

var expReference = infinity.MustParseHexAddress("891a1d1c8436c792d02fc2e8883fef7ab387eaeaacd25aa9f518be7be7856d54")

func TestFeed_Get(t *testing.T) {
	var (
		feedResource = func(owner, topic, at string) string {
			if at != "" {
				return fmt.Sprintf("/feeds/%s/%s?at=%s", owner, topic, at)
			}
			return fmt.Sprintf("/feeds/%s/%s", owner, topic)
		}
		mockStatestore = statestore.NewStateStore()
		logger         = logging.New(ioutil.Discard, 0)
		tag            = tags.NewTags(mockStatestore, logger)
		mockStorer     = mock.NewStorer()
		client, _, _   = newTestServer(t, testServerOptions{
			Storer: mockStorer,
			Tags:   tag,
		})
	)

	t.Run("malformed owner", func(t *testing.T) {
		jsonhttptest.Request(t, client, http.MethodGet, feedResource("xyz", "cc", ""), http.StatusBadRequest,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: "bad owner",
				Code:    http.StatusBadRequest,
			}),
		)
	})

	t.Run("malformed topic", func(t *testing.T) {
		jsonhttptest.Request(t, client, http.MethodGet, feedResource("8d3766440f0d7b949a5e32995d09619a7f86e632", "xxzzyy", ""), http.StatusBadRequest,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: "bad topic",
				Code:    http.StatusBadRequest,
			}),
		)
	})

	t.Run("at malformed", func(t *testing.T) {
		jsonhttptest.Request(t, client, http.MethodGet, feedResource("8d3766440f0d7b949a5e32995d09619a7f86e632", "aabbcc", "unbekannt"), http.StatusBadRequest,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: "bad at",
				Code:    http.StatusBadRequest,
			}),
		)
	})

	t.Run("with at", func(t *testing.T) {
		var (
			timestamp    = int64(12121212)
			ch           = toChunk(t, uint64(timestamp), expReference.Bytes())
			look         = newMockLookup(12, 0, ch, nil, &id{}, &id{})
			factory      = newMockFactory(look)
			idBytes, _   = (&id{}).MarshalBinary()
			client, _, _ = newTestServer(t, testServerOptions{
				Storer: mockStorer,
				Tags:   tag,
				Feeds:  factory,
			})
		)

		respHeaders := jsonhttptest.Request(t, client, http.MethodGet, feedResource(ownerString, "aabbcc", "12"), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(api.FeedReferenceResponse{Reference: expReference}),
		)

		h := respHeaders[api.InfinityFeedIndexHeader]
		if len(h) == 0 {
			t.Fatal("expected Smart Chain feed index header to be set")
		}
		b, err := hex.DecodeString(h[0])
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, idBytes) {
			t.Fatalf("feed index header mismatch. got %v want %v", b, idBytes)
		}
	})

	t.Run("latest", func(t *testing.T) {
		var (
			timestamp  = int64(12121212)
			ch         = toChunk(t, uint64(timestamp), expReference.Bytes())
			look       = newMockLookup(-1, 2, ch, nil, &id{}, &id{})
			factory    = newMockFactory(look)
			idBytes, _ = (&id{}).MarshalBinary()

			client, _, _ = newTestServer(t, testServerOptions{
				Storer: mockStorer,
				Tags:   tag,
				Feeds:  factory,
			})
		)

		respHeaders := jsonhttptest.Request(t, client, http.MethodGet, feedResource(ownerString, "aabbcc", ""), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(api.FeedReferenceResponse{Reference: expReference}),
		)

		if h := respHeaders[api.InfinityFeedIndexHeader]; len(h) > 0 {
			b, err := hex.DecodeString(h[0])
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b, idBytes) {
				t.Fatalf("feed index header mismatch. got %v want %v", b, idBytes)
			}
		} else {
			t.Fatal("expected Smart Chain feed index header to be set")
		}
	})
}

func TestFeed_Post(t *testing.T) {
	// post to owner, tpoic, then expect a reference
	// get the reference from the store, unmarshal to a
	// manifest entry and make sure all metadata correct
	var (
		mockStatestore = statestore.NewStateStore()
		logger         = logging.New(ioutil.Discard, 0)
		tag            = tags.NewTags(mockStatestore, logger)
		topic          = "aabbcc"
		mockStorer     = mock.NewStorer()
		client, _, _   = newTestServer(t, testServerOptions{
			Storer: mockStorer,
			Tags:   tag,
			Logger: logger,
		})
	)

	t.Run("ok", func(t *testing.T) {
		url := fmt.Sprintf("/feeds/%s/%s?type=%s", ownerString, topic, "sequence")
		jsonhttptest.Request(t, client, http.MethodPost, url, http.StatusCreated,
			jsonhttptest.WithExpectedJSONResponse(api.FeedReferenceResponse{
				Reference: expReference,
			}),
		)

		ls := loadsave.New(mockStorer, storage.ModePutUpload, false)
		i, err := manifest.NewMantarayManifestReference(expReference, ls)
		if err != nil {
			t.Fatal(err)
		}
		e, err := i.Lookup(context.Background(), "/")
		if err != nil {
			t.Fatal(err)
		}

		meta := e.Metadata()
		if e := meta[api.FeedMetadataEntryOwner]; e != ownerString {
			t.Fatalf("owner mismatch. got %s want %s", e, ownerString)
		}
		if e := meta[api.FeedMetadataEntryTopic]; e != topic {
			t.Fatalf("topic mismatch. got %s want %s", e, topic)
		}
		if e := meta[api.FeedMetadataEntryType]; e != "Sequence" {
			t.Fatalf("type mismatch. got %s want %s", e, "Sequence")
		}
	})
}

type factoryMock struct {
	sequenceCalled bool
	epochCalled    bool
	feed           *feeds.Feed
	lookup         feeds.Lookup
}

func newMockFactory(mockLookup feeds.Lookup) *factoryMock {
	return &factoryMock{lookup: mockLookup}
}

func (f *factoryMock) NewLookup(t feeds.Type, feed *feeds.Feed) (feeds.Lookup, error) {
	switch t {
	case feeds.Sequence:
		f.sequenceCalled = true
	case feeds.Epoch:
		f.epochCalled = true
	}
	f.feed = feed
	return f.lookup, nil
}

type mockLookup struct {
	at, after int64
	chunk     infinity.Chunk
	err       error
	cur, next feeds.Index
}

func newMockLookup(at, after int64, ch infinity.Chunk, err error, cur, next feeds.Index) *mockLookup {
	return &mockLookup{at: at, after: after, chunk: ch, err: err, cur: cur, next: next}
}

func (l *mockLookup) At(_ context.Context, at, after int64) (infinity.Chunk, feeds.Index, feeds.Index, error) {
	if l.at == -1 {
		// shortcut to ignore the value in the call since time.Now() is a moving target
		return l.chunk, l.cur, l.next, nil
	}
	if at == l.at && after == l.after {
		return l.chunk, l.cur, l.next, nil
	}
	return nil, nil, nil, errors.New("no feed update found")
}

func toChunk(t *testing.T, at uint64, payload []byte) infinity.Chunk {
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, at)
	content := append(ts, payload...)

	s := testingsoc.GenerateMockSOC(t, content)
	return s.Chunk()
}

type id struct{}

func (i *id) MarshalBinary() ([]byte, error) {
	return []byte("accd"), nil
}

func (i *id) String() string {
	return "44237"
}

func (*id) Next(last int64, at uint64) feeds.Index {
	return &id{}
}
