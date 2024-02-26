/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package populator_machinery

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	cmg "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"k8s.io/apimachinery/pkg/types"
)

const (
	httpPattern            = "/metrics"
	addr                   = "localhost:0"
	processStartTimeMetric = "process_start_time_seconds"
)

func initMgr() *metricsManager {
	mgr := initMetrics()
	mgr.startListener(addr, httpPattern)

	return mgr
}

func TestNew(t *testing.T) {
	mgr := initMgr()
	defer mgr.stopListener()
}

func TestRecordMetrics(t *testing.T) {
	mgr := initMgr()
	srvAddr := "http://" + mgr.srv.Addr + httpPattern
	defer mgr.stopListener()
	pvcUID := types.UID("uid1")
	mgr.operationStart(pvcUID)
	time.Sleep(1100 * time.Millisecond)
	mgr.recordMetrics(pvcUID, "result1")

	expected :=
		`# HELP process_start_time_seconds [ALPHA] Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds 0
# HELP volume_populator_operation_seconds [ALPHA] Time taken by each populator operation
# TYPE volume_populator_operation_seconds histogram
volume_populator_operation_seconds_bucket{result="result1",le="0.1"} 0
volume_populator_operation_seconds_bucket{result="result1",le="0.25"} 0
volume_populator_operation_seconds_bucket{result="result1",le="0.5"} 0
volume_populator_operation_seconds_bucket{result="result1",le="1"} 0
volume_populator_operation_seconds_bucket{result="result1",le="2.5"} 1
volume_populator_operation_seconds_bucket{result="result1",le="5"} 1
volume_populator_operation_seconds_bucket{result="result1",le="10"} 1
volume_populator_operation_seconds_bucket{result="result1",le="15"} 1
volume_populator_operation_seconds_bucket{result="result1",le="30"} 1
volume_populator_operation_seconds_bucket{result="result1",le="60"} 1
volume_populator_operation_seconds_bucket{result="result1",le="120"} 1
volume_populator_operation_seconds_bucket{result="result1",le="300"} 1
volume_populator_operation_seconds_bucket{result="result1",le="600"} 1
volume_populator_operation_seconds_bucket{result="result1",le="+Inf"} 1
volume_populator_operation_seconds_sum{result="result1"} 1.100328407
volume_populator_operation_seconds_count{result="result1"} 1
# HELP volume_populator_operations_in_flight [ALPHA] Total number of operations in flight
# TYPE volume_populator_operations_in_flight gauge
volume_populator_operations_in_flight 0
`

	if err := verifyMetric(expected, srvAddr); err != nil {
		t.Errorf("failed testing [%v]", err)
	}
}

func TestInFlightMetric(t *testing.T) {
	inFlightCheckInterval = time.Millisecond * 50

	mgr := initMgr()
	defer mgr.stopListener()
	srvAddr := "http://" + mgr.srv.Addr + httpPattern

	pvcUID1 := types.UID("uid1")
	mgr.operationStart(pvcUID1)
	time.Sleep(500 * time.Millisecond)

	if err := verifyInFlightMetric(`volume_populator_operations_in_flight 1`, srvAddr); err != nil {
		t.Errorf("failed testing [%v]", err)
	}

	pvcUID2 := types.UID("uid2")
	mgr.operationStart(pvcUID2)
	time.Sleep(500 * time.Millisecond)

	if err := verifyInFlightMetric(`volume_populator_operations_in_flight 2`, srvAddr); err != nil {
		t.Errorf("failed testing [%v]", err)
	}

	//  Record, should be down to 1
	mgr.recordMetrics(pvcUID1, "result1")
	time.Sleep(500 * time.Millisecond)

	if err := verifyInFlightMetric(`volume_populator_operations_in_flight 1`, srvAddr); err != nil {
		t.Errorf("failed testing [%v]", err)
	}

	//  Start 50 operations, should be 51
	for i := 0; i < 50; i++ {
		pvcUID := types.UID(fmt.Sprintf("uid%d", 3+i))
		mgr.operationStart(pvcUID)
	}
	time.Sleep(500 * time.Millisecond)

	if err := verifyInFlightMetric(`volume_populator_operations_in_flight 51`, srvAddr); err != nil {
		t.Errorf("failed testing [%v]", err)
	}
}

func verifyInFlightMetric(expected string, srvAddr string) error {
	rsp, err := http.Get(srvAddr)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get response from serve: %s", http.StatusText(rsp.StatusCode))
	}
	r, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	if !strings.Contains(string(r), expected) {
		return fmt.Errorf("failed, not equal")
	}

	return nil
}

func verifyMetric(expected, srvAddr string) error {
	rsp, err := http.Get(srvAddr)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get response from serve: %s", http.StatusText(rsp.StatusCode))
	}
	r, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	format := expfmt.ResponseFormat(rsp.Header)
	expectedReader := strings.NewReader(expected)
	expectedDecoder := expfmt.NewDecoder(expectedReader, format)
	expectedMfs := []*cmg.MetricFamily{}
	for {
		mf := &cmg.MetricFamily{}
		if err := expectedDecoder.Decode(mf); err != nil {
			// return correctly if EOF
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		expectedMfs = append(expectedMfs, mf)
	}

	gotReader := strings.NewReader(string(r))
	gotDecoder := expfmt.NewDecoder(gotReader, format)
	gotMfs := []*cmg.MetricFamily{}
	for {
		mf := &cmg.MetricFamily{}
		if err := gotDecoder.Decode(mf); err != nil {
			// return correctly if  EOF
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		gotMfs = append(gotMfs, mf)
	}

	if !containsMetrics(expectedMfs, gotMfs) {
		return fmt.Errorf("failed testing, expected\n%s\n, got\n%s\n", expected, string(r))
	}

	return nil
}

// sortMfs, sorts metric families in alphabetical order by type.
// currently only supports counter and histogram
func sortMfs(mfs []*cmg.MetricFamily) []*cmg.MetricFamily {
	var sortedMfs []*cmg.MetricFamily

	// Sort first by type
	sort.Slice(mfs, func(i, j int) bool {
		return *mfs[i].Type < *mfs[j].Type
	})

	// Next, sort by length of name
	sort.Slice(mfs, func(i, j int) bool {
		return len(*mfs[i].Name) < len(*mfs[j].Name)
	})

	return sortedMfs
}

func containsMetrics(expectedMfs, gotMfs []*cmg.MetricFamily) bool {
	if len(gotMfs) != len(expectedMfs) {
		fmt.Printf("Not same length: expected and got metrics families: %v vs. %v\n", len(expectedMfs), len(gotMfs))
		return false
	}

	// sort metric families for deterministic comparison.
	sortedExpectedMfs := sortMfs(expectedMfs)
	sortedGotMfs := sortMfs(gotMfs)

	// compare expected vs. sorted actual metrics
	for k, got := range sortedGotMfs {
		matchCount := 0
		expected := sortedExpectedMfs[k]

		if (got.Name == nil || *(got.Name) != *(expected.Name)) ||
			(got.Type == nil || *(got.Type) != *(expected.Type)) ||
			(got.Help == nil || *(got.Help) != *(expected.Help)) {
			fmt.Printf("invalid header info: got: %v, expected: %v\n", *got.Name, *expected.Name)
			fmt.Printf("invalid header info: got: %v, expected: %v\n", *got.Type, *expected.Type)
			fmt.Printf("invalid header info: got: %v, expected: %v\n", *got.Help, *expected.Help)
			return false
		}

		numRecords := len(expected.Metric)
		if len(got.Metric) < numRecords {
			fmt.Printf("Not the same number of records: got.Metric: %v, numRecords: %v\n", len(got.Metric), numRecords)
			return false
		}
		for i := 0; i < len(got.Metric); i++ {
			for j := 0; j < numRecords; j++ {
				if got.Metric[i].Histogram == nil && expected.Metric[j].Histogram != nil ||
					got.Metric[i].Histogram != nil && expected.Metric[j].Histogram == nil {
					fmt.Printf("got metric and expected metric histogram type mismatch")
					return false
				}

				// labels should be the same
				if !reflect.DeepEqual(got.Metric[i].Label, expected.Metric[j].Label) {
					continue
				}

				// metric type specific checks
				switch {
				case got.Metric[i].Histogram != nil && expected.Metric[j].Histogram != nil:
					gh := got.Metric[i].Histogram
					eh := expected.Metric[j].Histogram
					if gh == nil || eh == nil {
						continue
					}
					if !reflect.DeepEqual(gh.Bucket, eh.Bucket) {
						fmt.Println("got and expected histogram bucket not equal")
						continue
					}

					// this is a sum record, expecting a latency which is more than the
					// expected one. If the sum is smaller than expected, it will be considered
					// as NOT a match
					if gh.SampleSum == nil || eh.SampleSum == nil || *(gh.SampleSum) < *(eh.SampleSum) {
						fmt.Println("difference in sample sum")
						continue
					}
					if gh.SampleCount == nil || eh.SampleCount == nil || *(gh.SampleCount) != *(eh.SampleCount) {
						fmt.Println("difference in sample count")
						continue
					}

				case got.Metric[i].Counter != nil && expected.Metric[j].Counter != nil:
					gc := got.Metric[i].Counter
					ec := expected.Metric[j].Counter
					if gc.Value == nil || *(gc.Value) != *(ec.Value) {
						fmt.Println("difference in counter values")
						continue
					}
				}

				// this is a match
				matchCount = matchCount + 1
				break
			}
		}

		if matchCount != numRecords {
			fmt.Printf("matchCount %v, numRecords %v\n", matchCount, numRecords)
			return false
		}
	}

	return true
}

func TestProcessStartTimeMetricExist(t *testing.T) {
	mgr := initMgr()
	defer mgr.stopListener()
	metricsFamilies, err := mgr.registry.Gather()
	if err != nil {
		t.Fatalf("Error fetching metrics: %v", err)
	}

	for _, metricsFamily := range metricsFamilies {
		if metricsFamily.GetName() == processStartTimeMetric {
			return
		}
		m := metricsFamily.GetMetric()
		if m[0].GetGauge().GetValue() <= 0 {
			t.Fatalf("Expected non zero timestamp for process start time")
		}
	}

	t.Fatalf("Metrics does not contain %v. Scraped content: %v", processStartTimeMetric, metricsFamilies)
}
