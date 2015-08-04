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

package etcdhelper

import (
	"fmt"
	"time"
	"sync"
	"strconv"
	"os"
	golog "log"
	"path"

	"github.com/coreos/flannel/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
)

// etcd error codes
const (
	KeyNotFound       = 100
	KeyAlreadyExists  = 105
	EventIndexCleared = 401
)

type EtcdConfig struct {
	Endpoints []string
	Keyfile   string
	Certfile  string
	CAFile    string
	Prefix    string
}

type EtcdHelper struct {
	mux     sync.Mutex
	cli     *etcd.Client
	etcdCfg *EtcdConfig
}

func init() {
	etcd.SetLogger(golog.New(os.Stderr, "go-etcd", golog.LstdFlags))
}

func newEtcdClient(c *EtcdConfig) (*etcd.Client, error) {
	if c.Keyfile != "" || c.Certfile != "" || c.CAFile != "" {
		return etcd.NewTLSClient(c.Endpoints, c.Certfile, c.Keyfile, c.CAFile)
	} else {
		return etcd.NewClient(c.Endpoints), nil
	}
}

func NewEtcdHelper(config *EtcdConfig) (*EtcdHelper, error) {
	h := &EtcdHelper{
		etcdCfg: config,
	}

	var err error
	h.cli, err = newEtcdClient(config)
	if err != nil {
		return nil, err
	}

	return h, nil	
}

func (h *EtcdHelper) Get(key string, recursive bool) (*etcd.Response, error) {
	fullkey := path.Join(h.etcdCfg.Prefix, key)
	return h.client().Get(fullkey, false, recursive)
}

func (h *EtcdHelper) Create(key string, data string, ttl uint64) (*etcd.Response, error) {
	fullkey := path.Join(h.etcdCfg.Prefix, key)
	resp, err := h.client().Create(fullkey, data, ttl)
	if err != nil {
		return nil, err
	}

	ensureExpiration(resp, ttl)
	return resp, nil
}

func (h *EtcdHelper) Set(key string, data string, ttl uint64) (*etcd.Response, error) {
	fullkey := path.Join(h.etcdCfg.Prefix, key)
	resp, err := h.client().Set(fullkey, data, ttl)
	if err != nil {
		return nil, err
	}

	ensureExpiration(resp, ttl)
	return resp, nil
}

func (h *EtcdHelper) Delete(key string, recursive bool) (*etcd.Response, error) {
	fullkey := path.Join(h.etcdCfg.Prefix, key)
	return h.client().Delete(fullkey, recursive)
}

type watchResp struct {
	resp *etcd.Response
	err  error
}

func (h *EtcdHelper) doWatch(ctx context.Context, watchpath string, since uint64) (*etcd.Response, error) {
	stop := make(chan bool)
	respCh := make(chan watchResp)

	go func() {
		for {
			key := path.Join(h.etcdCfg.Prefix, watchpath)
			rresp, err := h.client().RawWatch(key, since, true, nil, stop)

			if err != nil {
				respCh <- watchResp{nil, err}
				return
			}

			if len(rresp.Body) == 0 {
				// etcd timed out, go back but recreate the client as the underlying
				// http transport gets hosed (http://code.google.com/p/go/issues/detail?id=8648)
				h.resetClient()
				continue
			}

			resp, err := rresp.Unmarshal()
			respCh <- watchResp{resp, err}
		}
	}()

	select {
	case <-ctx.Done():
		close(stop)
		<-respCh // Wait for f to return.
		return nil, ctx.Err()
	case wr := <-respCh:
		return wr.resp, wr.err
	}
}

type WatchCursor struct {
	Index uint64
}

func (c WatchCursor) String() string {
	return strconv.FormatUint(c.Index, 10)
}

type WatchError interface {
	error
	DoReset() bool
}

type watchError struct {
	s string
	doReset bool
}

func (e *watchError) Error() string {
	return e.s
}

func (e *watchError) DoReset() bool {
	return e.doReset
}

func (h *EtcdHelper) Watch(ctx context.Context, watchpath string, cursor interface{}) (*etcd.Response, WatchError) {
	if cursor == nil {
		return nil, &watchError{ "", true }
	}

	nextIndex := uint64(0)

	if wc, ok := cursor.(WatchCursor); ok {
		nextIndex = wc.Index
	} else if s, ok := cursor.(string); ok {
		var err error
		nextIndex, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			return nil, &watchError{ fmt.Sprintf("failed to parse cursor: %v", err), false }
		}
	} else {
		return nil, &watchError{ fmt.Sprintf("internal error: watch cursor is of unknown type"), false }
	}

	resp, err := h.doWatch(ctx, watchpath, nextIndex)
	switch {
	case err == nil:
		return resp, nil

	case isIndexTooSmall(err):
		log.Warning("Watch of subnet leases failed because etcd index outside history window")
		return nil, &watchError{ "", true }

	default:
		return nil, &watchError{ err.Error(), false }
	}
}

func isIndexTooSmall(err error) bool {
	etcdErr, ok := err.(*etcd.EtcdError)
	return ok && etcdErr.ErrorCode == EventIndexCleared
}

func (h *EtcdHelper) client() *etcd.Client {
	h.mux.Lock()
	defer h.mux.Unlock()
	return h.cli
}

func (h *EtcdHelper) resetClient() {
	h.mux.Lock()
	defer h.mux.Unlock()

	var err error
	h.cli.Close()
	h.cli, err = newEtcdClient(h.etcdCfg)
	if err != nil {
		panic(fmt.Errorf("resetClient: error recreating etcd client: %v", err))
	}
}

func ensureExpiration(resp *etcd.Response, ttl uint64) {
	if resp.Node.Expiration == nil {
		// should not be but calc it ourselves in this case
		log.Info("Expiration field missing on etcd response, calculating locally")
		exp := time.Now().Add(time.Duration(ttl) * time.Second)
		resp.Node.Expiration = &exp
	}
}
