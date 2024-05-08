// Copyright 2020 The Smart Chain Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debugapi_test

import (
	"net/http"
	"testing"

	"github.com/yanhuangpai/voyager"
	"github.com/yanhuangpai/voyager/pkg/debugapi"
	"github.com/yanhuangpai/voyager/pkg/jsonhttp/jsonhttptest"
)

func TestHealth(t *testing.T) {
	testServer := newTestServer(t, testServerOptions{})

	jsonhttptest.Request(t, testServer.Client, http.MethodGet, "/health", http.StatusOK,
		jsonhttptest.WithExpectedJSONResponse(debugapi.StatusResponse{
			Status:  "ok",
			Version: voyager.Version,
		}),
	)
}

func TestReadiness(t *testing.T) {
	testServer := newTestServer(t, testServerOptions{})

	jsonhttptest.Request(t, testServer.Client, http.MethodGet, "/readiness", http.StatusOK,
		jsonhttptest.WithExpectedJSONResponse(debugapi.StatusResponse{
			Status:  "ok",
			Version: voyager.Version,
		}),
	)
}
