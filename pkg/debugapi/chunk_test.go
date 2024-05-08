// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp/jsonhttptest"
	"github.com/yanhuangpai/voyager/pkg/storage"
	"github.com/yanhuangpai/voyager/pkg/storage/mock"
)

func TestHasChunkHandler(t *testing.T) {
	mockStorer := mock.NewStorer()
	testServer := newTestServer(t, testServerOptions{
		Storer: mockStorer,
	})

	key := infinity.MustParseHexAddress("aabbcc")
	value := []byte("data data data")

	_, err := mockStorer.Put(context.Background(), storage.ModePutUpload, infinity.NewChunk(key, value))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ok", func(t *testing.T) {
		jsonhttptest.Request(t, testServer.Client, http.MethodGet, "/chunks/"+key.String(), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusOK),
				Code:    http.StatusOK,
			}),
		)
	})

	t.Run("not found", func(t *testing.T) {
		jsonhttptest.Request(t, testServer.Client, http.MethodGet, "/chunks/abbbbb", http.StatusNotFound,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusNotFound),
				Code:    http.StatusNotFound,
			}),
		)
	})

	t.Run("bad address", func(t *testing.T) {
		jsonhttptest.Request(t, testServer.Client, http.MethodGet, "/chunks/abcd1100zz", http.StatusBadRequest,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: "bad address",
				Code:    http.StatusBadRequest,
			}),
		)
	})

	t.Run("remove-chunk", func(t *testing.T) {
		jsonhttptest.Request(t, testServer.Client, http.MethodDelete, "/chunks/"+key.String(), http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusOK),
				Code:    http.StatusOK,
			}),
		)
		yes, err := mockStorer.Has(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if yes {
			t.Fatalf("The chunk %s is not deleted", key.String())
		}
	})

	t.Run("remove-not-present-chunk", func(t *testing.T) {
		notPresentChunkAddress := "deadvoyagerf"
		jsonhttptest.Request(t, testServer.Client, http.MethodDelete, "/chunks/"+notPresentChunkAddress, http.StatusOK,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: http.StatusText(http.StatusOK),
				Code:    http.StatusOK,
			}),
		)
		yes, err := mockStorer.Has(context.Background(), infinity.NewAddress([]byte(notPresentChunkAddress)))
		if err != nil {
			t.Fatal(err)
		}
		if yes {
			t.Fatalf("The chunk %s is not deleted", notPresentChunkAddress)
		}
	})
}
