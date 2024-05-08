// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api_test

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/yanhuangpai/voyager/pkg/api"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp/jsonhttptest"
	"github.com/yanhuangpai/voyager/pkg/logging"
	statestore "github.com/yanhuangpai/voyager/pkg/statestore/mock"
	"github.com/yanhuangpai/voyager/pkg/storage/mock"
	"github.com/yanhuangpai/voyager/pkg/tags"
	"github.com/yanhuangpai/voyager/pkg/traversal"
)

func TestPinIfiHandler(t *testing.T) {
	var (
		dirUploadResource     = "/dirs"
		pinIfiResource        = "/pin/ifi"
		pinIfiAddressResource = func(addr string) string { return pinIfiResource + "/" + addr }
		pinChunksResource     = "/pin/chunks"

		mockStorer       = mock.NewStorer()
		mockStatestore   = statestore.NewStateStore()
		traversalService = traversal.NewService(mockStorer)
		logger           = logging.New(ioutil.Discard, 0)
		client, _, _     = newTestServer(t, testServerOptions{
			Storer:    mockStorer,
			Traversal: traversalService,
			Tags:      tags.NewTags(mockStatestore, logger),
		})
	)

	t.Run("pin-ifi-1", func(t *testing.T) {
		files := []f{
			{
				data: []byte("<h1>Infinity"),
				name: "index.html",
				dir:  "",
			},
		}

		tarReader := tarFiles(t, files)

		rootHash := "a85aaea6a34a5c7127a3546196f2111f866fe369c6d6562ed5d3313a99388c03"

		// verify directory tar upload response
		jsonhttptest.Request(t, client, http.MethodPost, dirUploadResource, http.StatusOK,
			jsonhttptest.WithRequestBody(tarReader),
			jsonhttptest.WithRequestHeader("Content-Type", api.ContentTypeTar),
			jsonhttptest.WithExpectedJSONResponse(api.FileUploadResponse{
				Reference: infinity.MustParseHexAddress(rootHash),
			}),
		)

		jsonhttptest.Request(t, client, http.MethodPost, pinIfiAddressResource(rootHash), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusOK),
				Code:    http.StatusOK,
			}),
		)

		expectedChunkCount := 7

		// get the reference as everytime it will change because of random encryption key
		var resp api.ListPinnedChunksResponse

		jsonhttptest.Request(t, client, http.MethodGet, pinChunksResource, http.StatusOK,
			jsonhttptest.WithUnmarshalJSONResponse(&resp),
		)

		if expectedChunkCount != len(resp.Chunks) {
			t.Fatalf("expected to find %d pinned chunks, got %d", expectedChunkCount, len(resp.Chunks))
		}
	})

	t.Run("unpin-ifi-1", func(t *testing.T) {
		rootHash := "a85aaea6a34a5c7127a3546196f2111f866fe369c6d6562ed5d3313a99388c03"

		jsonhttptest.Request(t, client, http.MethodDelete, pinIfiAddressResource(rootHash), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusOK),
				Code:    http.StatusOK,
			}),
		)

		jsonhttptest.Request(t, client, http.MethodGet, pinChunksResource, http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(api.ListPinnedChunksResponse{
				Chunks: []api.PinnedChunk{},
			}),
		)
	})

}
