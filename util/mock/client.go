// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mock

import (
	"context"

	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/util/memory"
)

// Client implement kv.Client interface, mocked from "CopClient" defined in
// "store/tikv/copprocessor.go".
type Client struct {
	kv.RequestTypeSupportedChecker
	MockResponse kv.Response
}

// Send implement kv.Client interface.
func (c *Client) Send(ctx context.Context, req *kv.Request, kv interface{}, sessionMemTracker *memory.Tracker, enabledRateLimit bool) kv.Response {
	return c.MockResponse
}
