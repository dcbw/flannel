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

package network

import (
	golog "log"
	"os"
	"path"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/etcd"
)

type Registry interface {
	getConfig(ctx context.Context, network string) (*etcd.Response, error)
	getSubnets(ctx context.Context, network string) (*etcd.Response, error)
	createSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error)
	updateSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error)
	deleteSubnet(ctx context.Context, network, sn string) (*etcd.Response, error)
	watchSubnets(ctx context.Context, network string, cursor interface{}) (*etcd.Response, etcdhelper.WatchError)
}

type etcdSubnetRegistry struct {
	cli *etcdhelper.EtcdHelper
}

func init() {
	etcd.SetLogger(golog.New(os.Stderr, "go-etcd", golog.LstdFlags))
}

func newEtcdSubnetRegistry(etcdHelper *etcdhelper.EtcdHelper) Registry {
	return &etcdSubnetRegistry{
		cli: etcdHelper,
	}
}

func (esr *etcdSubnetRegistry) getConfig(ctx context.Context, network string) (*etcd.Response, error) {
	return esr.cli.Get(path.Join(network, "config"), false)
}

func (esr *etcdSubnetRegistry) getSubnets(ctx context.Context, network string) (*etcd.Response, error) {
	return esr.cli.Get(path.Join(network, "subnets"), true)
}

func (esr *etcdSubnetRegistry) createSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error) {
	return esr.cli.Create(path.Join(network, "subnets", sn), data, ttl)
}

func (esr *etcdSubnetRegistry) updateSubnet(ctx context.Context, network, sn, data string, ttl uint64) (*etcd.Response, error) {
	return esr.cli.Set(path.Join(network, "subnets", sn), data, ttl)
}

func (esr *etcdSubnetRegistry) deleteSubnet(ctx context.Context, network, sn string) (*etcd.Response, error) {
	return esr.cli.Delete(path.Join(network, "subnets", sn), false)
}

func (esr *etcdSubnetRegistry) watchSubnets(ctx context.Context, network string, cursor interface{}) (*etcd.Response, etcdhelper.WatchError) {
	return esr.cli.Watch(ctx, path.Join(network, "subnets"), cursor)
}
