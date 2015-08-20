// Copyright 2015 CoreOS, Inc.
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

package ovs

import (
	"encoding/json"
	"fmt"

	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/flannel/subnet"
)

type network struct {
	name  string
	lease *subnet.Lease
	vni   int
	mtu   int
	be    *OVSBackend
}

func newNetwork(netname string, config *subnet.Config, mtu int, lease *subnet.Lease, be *OVSBackend) (*network, error) {
	n := &network{
		lease: lease,
		name:  netname,
		mtu:   mtu,
		be:    be,
	}

	var err error
	if netname != CLUSTER_NETWORK_NAME {
		n.vni, err = parseVniNetworkConfig(config)
		if err != nil {
			return nil, err
		}
	}

	return n, nil
}

func parseVniNetworkConfig(config *subnet.Config) (int, error) {
	if len(config.Backend) == 0 {
		return defaultVNI, nil
	}

	var data struct {
		VNI int
	}

	if err := json.Unmarshal(config.Backend, &data); err != nil {
		return 0, fmt.Errorf("failed to decode VNI: %v", err)
	}
	return data.VNI, nil
}

func (n *network) Lease() *subnet.Lease {
	return n.lease
}

func (n *network) MTU() int {
	return n.mtu
}

func (n *network) Run(ctx context.Context) {
	<-ctx.Done()
	n.be.removeNetwork(n)
}

func (n *network) SubnetFileVars() map[string]string {
	if n.name == CLUSTER_NETWORK_NAME {
		return nil
	}

	vars := make(map[string]string)
	vars["FLANNEL_OVS_VNID"] = fmt.Sprintf("%d", n.vni)
	return vars
}
