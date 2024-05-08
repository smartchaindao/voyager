// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	statestore "github.com/yanhuangpai/voyager/pkg/statestore/mock"

	"github.com/yanhuangpai/voyager/pkg/api"
	"github.com/yanhuangpai/voyager/pkg/infinity"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp/jsonhttptest"
	"github.com/yanhuangpai/voyager/pkg/logging"
	"github.com/yanhuangpai/voyager/pkg/storage/mock"
	"github.com/yanhuangpai/voyager/pkg/tags"
	mockbytes "gitlab.com/nolash/go-mockbytes"
)

// TestBytes tests that the data upload api responds as expected when uploading,
// downloading and requesting a resource that cannot be found.
func TestBytes(t *testing.T) {
	var (
		resource       = "/bytes"
		targets        = "0x222"
		expHash        = "29a5fb121ce96194ba8b7b823a1f9c6af87e1791f824940a53b5a7efe3f790d9"
		mockStorer     = mock.NewStorer()
		mockStatestore = statestore.NewStateStore()
		logger         = logging.New(ioutil.Discard, 0)
		client, _, _   = newTestServer(t, testServerOptions{
			Storer: mockStorer,
			Tags:   tags.NewTags(mockStatestore, logger),
			Logger: logging.New(ioutil.Discard, 5),
		})
	)
	g := mockbytes.New(0, mockbytes.MockTypeStandard).WithModulus(255)
	content, err := g.SequentialBytes(infinity.ChunkSize * 2)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("upload", func(t *testing.T) {
		jsonhttptest.Request(t, client, http.MethodPost, resource, http.StatusOK,
			jsonhttptest.WithRequestBody(bytes.NewReader(content)),
			jsonhttptest.WithExpectedJSONResponse(api.BytesPostResponse{
				Reference: infinity.MustParseHexAddress(expHash),
			}),
		)
	})

	t.Run("download", func(t *testing.T) {
		resp := request(t, client, http.MethodGet, resource+"/"+expHash, nil, http.StatusOK)
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, content) {
			t.Fatalf("data mismatch. got %s, want %s", string(data), string(content))
		}
	})

	t.Run("download-with-targets", func(t *testing.T) {
		resp := request(t, client, http.MethodGet, resource+"/"+expHash+"?targets="+targets, nil, http.StatusOK)

		if resp.Header.Get(api.TargetsRecoveryHeader) != targets {
			t.Fatalf("targets mismatch. got %s, want %s", resp.Header.Get(api.TargetsRecoveryHeader), targets)
		}
	})

	t.Run("not found", func(t *testing.T) {
		jsonhttptest.Request(t, client, http.MethodGet, resource+"/abcd", http.StatusNotFound,
			jsonhttptest.WithExpectedJSONResponse(jsonhttp.StatusResponse{
				Message: "Not Found",
				Code:    http.StatusNotFound,
			}),
		)
	})
}
