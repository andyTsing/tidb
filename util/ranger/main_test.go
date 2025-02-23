// Copyright 2021 PingCAP, Inc.
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

package ranger_test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/pingcap/tidb/testkit/testdata"
	"github.com/pingcap/tidb/util/testbridge"
	"go.uber.org/goleak"
)

var testData testdata.TestData

func TestMain(m *testing.M) {
	testbridge.WorkaroundGoCheckFlags()

	flag.Parse()

	var err error
	testData, err = testdata.LoadTestSuiteData("testdata", "ranger_suite")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "testdata: Errors on loading test data from file: %v\n", err)
		os.Exit(1)
	}

	if exitCode := m.Run(); exitCode != 0 {
		os.Exit(exitCode)
	}

	err = testData.GenerateOutputIfNeeded()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "testdata: Errors on generating output: %v\n", err)
		os.Exit(1)
	}

	opts := []goleak.Option{
		goleak.IgnoreTopFunction("go.etcd.io/etcd/pkg/logutil.(*MergeLogger).outputLoop"),
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
	}

	if err := goleak.Find(opts...); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "goleak: Errors on successful test run: %v\n", err)
		os.Exit(1)
	}
}
