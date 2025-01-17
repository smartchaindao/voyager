// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package retrieval_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	accountingmock "github.com/yanhuangpai/voyager/pkg/accounting/mock"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/p2p"
	"github.com/yanhuangpai/voyager/pkg/p2p/protobuf"
	"github.com/yanhuangpai/voyager/pkg/p2p/streamtest"
	"github.com/yanhuangpai/voyager/pkg/retrieval"
	pb "github.com/yanhuangpai/voyager/pkg/retrieval/pb"
	"github.com/yanhuangpai/voyager/pkg/storage"
	storemock "github.com/yanhuangpai/voyager/pkg/storage/mock"
	testingc "github.com/yanhuangpai/voyager/pkg/storage/testing"
	"github.com/yanhuangpai/voyager/pkg/topology"
)

var testTimeout = 5 * time.Second

// TestDelivery tests that a naive request -> delivery flow works.
func TestDelivery(t *testing.T) {
	var (
		logger               = logging.New(ioutil.Discard, 0)
		mockStorer           = storemock.NewStorer()
		chunk                = testingc.FixtureChunk("0033")
		clientMockAccounting = accountingmock.NewAccounting()
		serverMockAccounting = accountingmock.NewAccounting()
		clientAddr           = infinity.MustParseHexAddress("9ee7add8")
		serverAddr           = infinity.MustParseHexAddress("9ee7add7")

		price      = uint64(10)
		pricerMock = accountingmock.NewPricer(price, price)
	)
	// put testdata in the mock store of the server
	_, err := mockStorer.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	// create the server that will handle the request and will serve the response
	server := retrieval.New(infinity.MustParseHexAddress("0034"), mockStorer, nil, nil, logger, serverMockAccounting, pricerMock, nil)
	recorder := streamtest.New(
		streamtest.WithProtocols(server.Protocol()),
		streamtest.WithBaseAddr(clientAddr),
	)

	// client mock storer does not store any data at this point
	// but should be checked at at the end of the test for the
	// presence of the chunk address key and value to ensure delivery
	// was successful
	clientMockStorer := storemock.NewStorer()

	ps := mockPeerSuggester{eachPeerRevFunc: func(f topology.EachPeerFunc) error {
		_, _, _ = f(serverAddr, 0)
		return nil
	}}

	client := retrieval.New(clientAddr, clientMockStorer, recorder, ps, logger, clientMockAccounting, pricerMock, nil)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	v, err := client.RetrieveChunk(ctx, chunk.Address())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(v.Data(), chunk.Data()) {
		t.Fatalf("request and response data not equal. got %s want %s", v, chunk.Data())
	}
	records, err := recorder.Records(serverAddr, "retrieval", "1.0.0", "retrieval")
	if err != nil {
		t.Fatal(err)
	}
	if l := len(records); l != 1 {
		t.Fatalf("got %v records, want %v", l, 1)
	}

	record := records[0]

	messages, err := protobuf.ReadMessages(
		bytes.NewReader(record.In()),
		func() protobuf.Message { return new(pb.Request) },
	)
	if err != nil {
		t.Fatal(err)
	}
	var reqs []string
	for _, m := range messages {
		reqs = append(reqs, hex.EncodeToString(m.(*pb.Request).Addr))
	}

	if len(reqs) != 1 {
		t.Fatalf("got too many requests. want 1 got %d", len(reqs))
	}

	messages, err = protobuf.ReadMessages(
		bytes.NewReader(record.Out()),
		func() protobuf.Message { return new(pb.Delivery) },
	)
	if err != nil {
		t.Fatal(err)
	}
	var gotDeliveries []string
	for _, m := range messages {
		gotDeliveries = append(gotDeliveries, string(m.(*pb.Delivery).Data))
	}

	if len(gotDeliveries) != 1 {
		t.Fatalf("got too many deliveries. want 1 got %d", len(gotDeliveries))
	}

	clientBalance, _ := clientMockAccounting.Balance(serverAddr)
	if clientBalance.Int64() != -int64(price) {
		t.Fatalf("unexpected balance on client. want %d got %d", -price, clientBalance)
	}

	serverBalance, _ := serverMockAccounting.Balance(clientAddr)
	if serverBalance.Int64() != int64(price) {
		t.Fatalf("unexpected balance on server. want %d got %d", price, serverBalance)
	}
}

func TestRetrieveChunk(t *testing.T) {
	var (
		logger = logging.New(ioutil.Discard, 0)
		pricer = accountingmock.NewPricer(1, 1)
	)

	// requesting a chunk from downstream peer is expected
	t.Run("downstream", func(t *testing.T) {
		serverAddress := infinity.MustParseHexAddress("03")
		clientAddress := infinity.MustParseHexAddress("01")
		chunk := testingc.FixtureChunk("02c2")

		serverStorer := storemock.NewStorer()
		_, err := serverStorer.Put(context.Background(), storage.ModePutUpload, chunk)
		if err != nil {
			t.Fatal(err)
		}

		server := retrieval.New(serverAddress, serverStorer, nil, nil, logger, accountingmock.NewAccounting(), pricer, nil)
		recorder := streamtest.New(streamtest.WithProtocols(server.Protocol()))

		clientSuggester := mockPeerSuggester{eachPeerRevFunc: func(f topology.EachPeerFunc) error {
			_, _, _ = f(serverAddress, 0)
			return nil
		}}
		client := retrieval.New(clientAddress, nil, recorder, clientSuggester, logger, accountingmock.NewAccounting(), pricer, nil)

		got, err := client.RetrieveChunk(context.Background(), chunk.Address())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Data(), chunk.Data()) {
			t.Fatalf("got data %x, want %x", got.Data(), chunk.Data())
		}
	})

	t.Run("forward", func(t *testing.T) {
		chunk := testingc.FixtureChunk("0025")

		serverAddress := infinity.MustParseHexAddress("0100000000000000000000000000000000000000000000000000000000000000")
		forwarderAddress := infinity.MustParseHexAddress("0200000000000000000000000000000000000000000000000000000000000000")
		clientAddress := infinity.MustParseHexAddress("030000000000000000000000000000000000000000000000000000000000000000")

		serverStorer := storemock.NewStorer()
		_, err := serverStorer.Put(context.Background(), storage.ModePutUpload, chunk)
		if err != nil {
			t.Fatal(err)
		}

		server := retrieval.New(
			serverAddress,
			serverStorer, // chunk is in server's store
			nil,
			nil,
			logger,
			accountingmock.NewAccounting(),
			pricer,
			nil,
		)

		forwarder := retrieval.New(
			forwarderAddress,
			storemock.NewStorer(), // no chunk in forwarder's store
			streamtest.New(streamtest.WithProtocols(server.Protocol())), // connect to server
			mockPeerSuggester{eachPeerRevFunc: func(f topology.EachPeerFunc) error {
				_, _, _ = f(serverAddress, 0) // suggest server's address
				return nil
			}},
			logger,
			accountingmock.NewAccounting(),
			pricer,
			nil,
		)

		client := retrieval.New(
			clientAddress,
			storemock.NewStorer(), // no chunk in clients's store
			streamtest.New(streamtest.WithProtocols(forwarder.Protocol())), // connect to forwarder
			mockPeerSuggester{eachPeerRevFunc: func(f topology.EachPeerFunc) error {
				_, _, _ = f(forwarderAddress, 0) // suggest forwarder's address
				return nil
			}},
			logger,
			accountingmock.NewAccounting(),
			pricer,
			nil,
		)

		got, err := client.RetrieveChunk(context.Background(), chunk.Address())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Data(), chunk.Data()) {
			t.Fatalf("got data %x, want %x", got.Data(), chunk.Data())
		}
	})
}

func TestRetrievePreemptiveRetry(t *testing.T) {
	t.Skip("needs some more tendering. baseaddr change made a mess here")
	logger := logging.New(ioutil.Discard, 0)

	chunk := testingc.FixtureChunk("0025")
	someOtherChunk := testingc.FixtureChunk("0033")

	price := uint64(1)
	pricerMock := accountingmock.NewPricer(price, price)

	clientAddress := infinity.MustParseHexAddress("1010")

	serverAddress1 := infinity.MustParseHexAddress("1000000000000000000000000000000000000000000000000000000000000000")
	serverAddress2 := infinity.MustParseHexAddress("0200000000000000000000000000000000000000000000000000000000000000")
	peers := []infinity.Address{
		serverAddress1,
		serverAddress2,
	}

	serverStorer1 := storemock.NewStorer()
	serverStorer2 := storemock.NewStorer()

	// we put some other chunk on server 1
	_, err := serverStorer1.Put(context.Background(), storage.ModePutUpload, someOtherChunk)
	if err != nil {
		t.Fatal(err)
	}
	// we put chunk we need on server 2
	_, err = serverStorer2.Put(context.Background(), storage.ModePutUpload, chunk)
	if err != nil {
		t.Fatal(err)
	}

	noPeerSuggester := mockPeerSuggester{eachPeerRevFunc: func(f topology.EachPeerFunc) error {
		_, _, _ = f(infinity.ZeroAddress, 0)
		return nil
	}}

	peerSuggesterFn := func(peers ...infinity.Address) topology.EachPeerer {
		if len(peers) == 0 {
			return noPeerSuggester
		}

		peerID := 0
		peerSuggester := mockPeerSuggester{eachPeerRevFunc: func(f topology.EachPeerFunc) error {
			_, _, _ = f(peers[peerID], 0)
			// circulate suggested peers
			peerID++
			if peerID >= len(peers) {
				peerID = 0
			}
			return nil
		}}
		return peerSuggester
	}

	server1 := retrieval.New(serverAddress1, serverStorer1, nil, noPeerSuggester, logger, accountingmock.NewAccounting(), pricerMock, nil)
	server2 := retrieval.New(serverAddress2, serverStorer2, nil, noPeerSuggester, logger, accountingmock.NewAccounting(), pricerMock, nil)

	t.Run("peer not reachable", func(t *testing.T) {
		recorder := streamtest.New(
			streamtest.WithProtocols(
				server1.Protocol(),
				server2.Protocol(),
			),
			streamtest.WithMiddlewares(
				func(h p2p.HandlerFunc) p2p.HandlerFunc {
					return func(ctx context.Context, peer p2p.Peer, stream p2p.Stream) error {
						// NOTE: return error for peer1
						if serverAddress1.Equal(peer.Address) {
							return fmt.Errorf("peer not reachable: %s", peer.Address.String())
						}

						if serverAddress2.Equal(peer.Address) {
							return server2.Handler(ctx, peer, stream)
						}

						return fmt.Errorf("unknown peer: %s", peer.Address.String())
					}
				},
			),
		)

		client := retrieval.New(clientAddress, nil, recorder, peerSuggesterFn(peers...), logger, accountingmock.NewAccounting(), pricerMock, nil)

		got, err := client.RetrieveChunk(context.Background(), chunk.Address())
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got.Data(), chunk.Data()) {
			t.Fatalf("got data %x, want %x", got.Data(), chunk.Data())
		}
	})

	t.Run("peer does not have chunk", func(t *testing.T) {
		recorder := streamtest.New(
			streamtest.WithProtocols(
				server1.Protocol(),
				server2.Protocol(),
			),
			streamtest.WithMiddlewares(
				func(h p2p.HandlerFunc) p2p.HandlerFunc {
					return func(ctx context.Context, peer p2p.Peer, stream p2p.Stream) error {
						if serverAddress1.Equal(peer.Address) {
							return server1.Handler(ctx, peer, stream)
						}

						if serverAddress2.Equal(peer.Address) {
							return server2.Handler(ctx, peer, stream)
						}

						return fmt.Errorf("unknown peer: %s", peer.Address.String())
					}
				},
			),
		)

		client := retrieval.New(clientAddress, nil, recorder, peerSuggesterFn(peers...), logger, accountingmock.NewAccounting(), pricerMock, nil)

		got, err := client.RetrieveChunk(context.Background(), chunk.Address())
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got.Data(), chunk.Data()) {
			t.Fatalf("got data %x, want %x", got.Data(), chunk.Data())
		}
	})

	t.Run("one peer is slower", func(t *testing.T) {
		serverStorer1 := storemock.NewStorer()
		serverStorer2 := storemock.NewStorer()

		// both peers have required chunk
		_, err := serverStorer1.Put(context.Background(), storage.ModePutUpload, chunk)
		if err != nil {
			t.Fatal(err)
		}
		_, err = serverStorer2.Put(context.Background(), storage.ModePutUpload, chunk)
		if err != nil {
			t.Fatal(err)
		}

		server1MockAccounting := accountingmock.NewAccounting()
		server2MockAccounting := accountingmock.NewAccounting()

		server1 := retrieval.New(serverAddress1, serverStorer1, nil, noPeerSuggester, logger, server1MockAccounting, pricerMock, nil)
		server2 := retrieval.New(serverAddress2, serverStorer2, nil, noPeerSuggester, logger, server2MockAccounting, pricerMock, nil)

		// NOTE: must be more than retry duration
		// (here one second more)
		server1ResponseDelayDuration := 6 * time.Second

		recorder := streamtest.New(
			streamtest.WithProtocols(
				server1.Protocol(),
				server2.Protocol(),
			),
			streamtest.WithMiddlewares(
				func(h p2p.HandlerFunc) p2p.HandlerFunc {
					return func(ctx context.Context, peer p2p.Peer, stream p2p.Stream) error {
						if serverAddress1.Equal(peer.Address) {
							// NOTE: sleep time must be more than retry duration
							time.Sleep(server1ResponseDelayDuration)
							return server1.Handler(ctx, peer, stream)
						}

						if serverAddress2.Equal(peer.Address) {
							return server2.Handler(ctx, peer, stream)
						}

						return fmt.Errorf("unknown peer: %s", peer.Address.String())
					}
				},
			),
		)

		clientMockAccounting := accountingmock.NewAccounting()

		client := retrieval.New(clientAddress, nil, recorder, peerSuggesterFn(peers...), logger, clientMockAccounting, pricerMock, nil)

		got, err := client.RetrieveChunk(context.Background(), chunk.Address())
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got.Data(), chunk.Data()) {
			t.Fatalf("got data %x, want %x", got.Data(), chunk.Data())
		}

		clientServer1Balance, _ := clientMockAccounting.Balance(serverAddress1)
		if clientServer1Balance.Int64() != 0 {
			t.Fatalf("unexpected balance on client. want %d got %d", -price, clientServer1Balance)
		}

		clientServer2Balance, _ := clientMockAccounting.Balance(serverAddress2)
		if clientServer2Balance.Int64() != -int64(price) {
			t.Fatalf("unexpected balance on client. want %d got %d", -price, clientServer2Balance)
		}

		// wait and check balance again
		// (yet one second more than before, minus original duration)
		time.Sleep(2 * time.Second)

		clientServer1Balance, _ = clientMockAccounting.Balance(serverAddress1)
		if clientServer1Balance.Int64() != -int64(price) {
			t.Fatalf("unexpected balance on client. want %d got %d", -price, clientServer1Balance)
		}

		clientServer2Balance, _ = clientMockAccounting.Balance(serverAddress2)
		if clientServer2Balance.Int64() != -int64(price) {
			t.Fatalf("unexpected balance on client. want %d got %d", -price, clientServer2Balance)
		}
	})

	t.Run("peer forwards request", func(t *testing.T) {
		// server 2 has the chunk
		server2 := retrieval.New(serverAddress2, serverStorer2, nil, noPeerSuggester, logger, accountingmock.NewAccounting(), pricerMock, nil)

		server1Recorder := streamtest.New(
			streamtest.WithProtocols(server2.Protocol()),
		)

		// server 1 will forward request to server 2
		server1 := retrieval.New(serverAddress1, serverStorer1, server1Recorder, peerSuggesterFn(serverAddress2), logger, accountingmock.NewAccounting(), pricerMock, nil)

		clientRecorder := streamtest.New(
			streamtest.WithProtocols(server1.Protocol()),
		)

		// client only knows about server 1
		client := retrieval.New(clientAddress, nil, clientRecorder, peerSuggesterFn(serverAddress1), logger, accountingmock.NewAccounting(), pricerMock, nil)

		got, err := client.RetrieveChunk(context.Background(), chunk.Address())
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got.Data(), chunk.Data()) {
			t.Fatalf("got data %x, want %x", got.Data(), chunk.Data())
		}
	})
}

type mockPeerSuggester struct {
	eachPeerRevFunc func(f topology.EachPeerFunc) error
}

func (s mockPeerSuggester) EachPeer(topology.EachPeerFunc) error {
	return errors.New("not implemented")
}
func (s mockPeerSuggester) EachPeerRev(f topology.EachPeerFunc) error {
	return s.eachPeerRevFunc(f)
}
