// Copyright 2022 The etcd Authors
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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestMoveLeader(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []testCase{
		{
			name: "Secure/ClientAutoTLS/PeerAutoTLS",
			config: config.NewClusterConfig(
				config.WithClientTLS(config.AutoTLS),
				config.WithPeerTLS(config.AutoTLS)),
		},
		{
			name: "Secure/ClientAutoTLS/ManualTLS",
			config: config.NewClusterConfig(
				config.WithClientTLS(config.AutoTLS),
				config.WithPeerTLS(config.ManualTLS)),
		},
		{
			name: "Secure/ClientManualTLS/PeerAutoTLS",
			config: config.NewClusterConfig(
				config.WithClientTLS(config.ManualTLS),
				config.WithPeerTLS(config.AutoTLS)),
		},
		{
			name: "Secure/ClientManualTLS/ManualTLS",
			config: config.NewClusterConfig(
				config.WithClientTLS(config.ManualTLS),
				config.WithPeerTLS(config.ManualTLS)),
		},
		{
			name:   "Insecure",
			config: config.DefaultClusterConfig(),
		},
	}

	nestedCases := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "happy path",
			env:  map[string]string{},
		},
		{
			name: "with env",
			env:  map[string]string{"ETCDCTL_ENDPOINTS": "something-else-is-set"},
		},
	}

	for _, tc := range tcs {
		for _, nc := range nestedCases {
			t.Run(tc.name+"/"+nc.name, func(t *testing.T) {
				testMoveLeader(t, tc.config, nc.env)
			})
		}
	}
}

func testMoveLeader(t *testing.T, cfg config.ClusterConfig, envVars map[string]string) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(cfg))
	defer func() {
		if err := clus.Close(); err != nil {
			t.Fatalf("TestMoveLeader failed: got error when closing cluster, err:%v", err)
		}
	}()
	cc := testutils.MustClient(clus.Client())

	leadIdx := clus.WaitLeader(t)
	leadc := clus.Members()[leadIdx].Client()
	leadStatus, err := leadc.Status(ctx)
	if err != nil {
		return
	}
	leadId := leadStatus[0].Leader

	transfreec := clus.Members()[(leadIdx+1)%cfg.ClusterSize].Client()

	cx := ctlCtx{
		t:           t,
		cfg:         *e2e.NewConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		epc:         epc,
		envMap:      envVars,
	}

	tests := []struct {
		eps    []string
		expect string
	}{
		{ // request to non-leader
			[]string{cx.epc.EndpointsV3()[(leadIdxx+1)%3]},
			"no leader endpoint given at ",
		},
		{ // request to leader
			[]string{cx.epc.EndpointsV3()[leadIdxx]},
			fmt.Sprintf("Leadership transferred from %s to %s", types.ID(leaderID), types.ID(transferee)),
		},
		{ // request to all endpoints
			cx.epc.EndpointsV3(),
			fmt.Sprintf("Leadership transferred"),
		},
	}
	for i, tc := range tests {
		prefix := cx.prefixArgs(tc.eps)
		cmdArgs := append(prefix, "move-leader", types.ID(transferee).String())
		if err := e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, tc.expect); err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
	}
}

func setupEtcdctlTest(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, quorum bool) *e2e.EtcdProcessCluster {
	if !quorum {
		cfg = e2e.ConfigStandalone(*cfg)
	}
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t, cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}
