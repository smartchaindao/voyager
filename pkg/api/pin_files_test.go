// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sort"
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

func TestPinFilesHandler(t *testing.T) {
	var (
		fileUploadResource      = "/files"
		pinFilesResource        = "/pin/files"
		pinFilesAddressResource = func(addr string) string { return pinFilesResource + "/" + addr }
		pinChunksResource       = "/pin/chunks"

		simpleData = []byte("this is a simple text")

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

	t.Run("pin-file-1", func(t *testing.T) {
		rootHash := "dc82503e0ed041a57327ad558d7aa69a867024c8221306c461ae359dc34d1c6a"
		metadataHash := "d936d7180f230b3424842ea10848aa205f2f0e830cb9cc7588a39c9381544bf9"
		contentHash := "838d0a193ecd1152d1bb1432d5ecc02398533b2494889e23b8bd5ace30ac2aeb"

		jsonhttptest.Request(t, client, http.MethodPost, fileUploadResource, http.StatusOK,
			jsonhttptest.WithRequestBody(bytes.NewReader(simpleData)),
			jsonhttptest.WithExpectedJSONResponse(api.FileUploadResponse{
				Reference: infinity.MustParseHexAddress(rootHash),
			}),
			jsonhttptest.WithRequestHeader("Content-Type", "text/plain"),
		)

		jsonhttptest.Request(t, client, http.MethodPost, pinFilesAddressResource(rootHash), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusOK),
				Code:    http.StatusOK,
			}),
		)

		hashes := []string{rootHash, metadataHash, contentHash}
		sort.Strings(hashes)

		expectedResponse := api.ListPinnedChunksResponse{
			Chunks: []api.PinnedChunk{},
		}

		for _, h := range hashes {
			expectedResponse.Chunks = append(expectedResponse.Chunks, api.PinnedChunk{
				Address:    infinity.MustParseHexAddress(h),
				PinCounter: 1,
			})
		}

		jsonhttptest.Request(t, client, http.MethodGet, pinChunksResource, http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(expectedResponse),
		)
	})

	t.Run("unpin-file-1", func(t *testing.T) {
		rootHash := "dc82503e0ed041a57327ad558d7aa69a867024c8221306c461ae359dc34d1c6a"

		jsonhttptest.Request(t, client, http.MethodDelete, pinFilesAddressResource(rootHash), http.StatusOK,
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
