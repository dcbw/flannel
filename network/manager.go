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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/subnet"
)

type (
	EventType int

	Event struct {
		Type EventType `json:"type"`
		Name string     `json:"name"`
	}
)

const (
	NetworkAdded EventType = iota
	NetworkRemoved
)

type WatchResult struct {
	// Either Events or Snapshot should be set.
	// If Events is not empty, it means the cursor
	// was out of range and Snapshot contains the current
	// list of network names
	Events   []Event     `json:"events"`
	Snapshot []string    `json:"snapshot"`
	Cursor   interface{} `json:"cursor"`
}

func (et EventType) MarshalJSON() ([]byte, error) {
	s := ""

	switch et {
	case NetworkAdded:
		s = "added"
	case NetworkRemoved:
		s = "removed"
	default:
		return nil, errors.New("bad event type")
	}
	return json.Marshal(s)
}

func (et *EventType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "\"added\"":
		*et = NetworkAdded
	case "\"removed\"":
		*et = NetworkRemoved
	default:
		fmt.Println(string(data))
		return errors.New("bad event type")
	}

	return nil
}

type NetworkManager interface {
	Init(ctx context.Context, sm subnet.Manager, netnames []string)
	WatchNetworks(ctx context.Context, cursor interface{}) (WatchResult, error)
}
