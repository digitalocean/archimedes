//   Copyright 2020 DigitalOcean
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package archimedes

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/ceph/go-ceph/rados"
)

// CephClient provides an abstraction for client calls
// made into Ceph.
type CephClient interface {
	// BackfillingPGs surfaces the list of PGs that are either
	// in 'backfilling' or 'backfill_weight' state.
	BackfillingPGs() (int, error)

	// RecoveringPGs surfaces the list of PGs that are either
	// in 'recovering' or 'recovery_weight' state.
	RecoveringPGs() (int, error)

	// OSDTree returns a parsed version of `ceph osd tree`.
	OSDTree() (*OSDTreeOut, error)

	// CrushReweight updates the given OSD to the crush reweight
	// value provided.
	CrushReweight(osdID int, crushWeight float64) error

	// EnableCephBalancer enables the Ceph balancer.
	EnableCephBalancer() error

	// Close is used to disconnect Ceph connection once used.
	Close()
}

type cephClient struct {
	conn *rados.Conn
}

func (c *cephClient) BackfillingPGs() (int, error) {
	return c.getPGsByState("backfilling", "backfill_wait")
}

func (c *cephClient) RecoveringPGs() (int, error) {
	return c.getPGsByState("recovering", "recovery_wait")
}

func (c *cephClient) getPGsByState(states ...string) (int, error) {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "status",
		"format": "json",
	})
	if err != nil {
		return 0, err
	}

	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return 0, err
	}

	stats := &healthStats{}
	if err := json.Unmarshal(buf, stats); err != nil {
		return 0, err
	}

	var count int
	for _, p := range stats.PGMap.PGsByState {
		for _, state := range states {
			if strings.Contains(p.States, state) {
				count += int(p.Count)
			}
		}
	}

	return count, nil
}

func (c *cephClient) OSDTree() (*OSDTreeOut, error) {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd tree",
		"format": "json",
	})
	if err != nil {
		return nil, err
	}

	buf, _, err := c.conn.MonCommand(cmd)
	if err != nil {
		return nil, err
	}

	ost := &OSDTreeOut{}
	if err := json.Unmarshal(buf, ost); err != nil {
		return nil, err
	}

	return ost, nil
}

func (c *cephClient) CrushReweight(osdID int, crushWeight float64) error {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "osd crush reweight",
		"name":   fmt.Sprintf("osd.%d", osdID),
		"weight": crushWeight,
	})
	if err != nil {
		return err
	}

	_, _, err = c.conn.MonCommand(cmd)
	return err
}

func (c *cephClient) EnableCephBalancer() error {
	cmd, err := json.Marshal(map[string]interface{}{
		"prefix": "balancer on",
	})
	if err != nil {
		return err
	}

	_, _, err = c.conn.MgrCommand([][]byte{cmd})
	return err
}

func (c *cephClient) Close() {
	c.conn.Shutdown()
}

// Verify compile time that `cephClient` implements `CephClient`.
var _ CephClient = &cephClient{}

// NewCephClient takes in Ceph user and path to ceph.conf for
// establishing a connection to ceph cluster and returning a
// usable handle.
func NewCephClient(user, configPath string) (CephClient, error) {
	// The cluster name can always be derived from the /etc/ceph/<cluster>.conf
	confParts := strings.SplitN(path.Base(configPath), ".", 2)
	if len(confParts) < 2 {
		return nil, fmt.Errorf("invalid ceph conf: %q", configPath)
	}
	clusterName := confParts[0]

	conn, err := rados.NewConnWithClusterAndUser(clusterName, user)
	if err != nil {
		return nil, fmt.Errorf("cannot create conn stub (user=%q,cluster=%q): %s", user, clusterName, err)
	}

	err = conn.ReadConfigFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file %q: %s", configPath, err)
	}

	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("error connecting to cluster: %s", err)
	}

	return &cephClient{
		conn: conn,
	}, nil
}

// OSDTreeOut provides a representation for output of
// `ceph osd tree -f json`.
type OSDTreeOut struct {
	Nodes []nodeType `json:"nodes"`
	Stray []nodeType `json:"stray"`
}

type nodeType struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	Reweight    float64 `json:"reweight"`
	CrushWeight float64 `json:"crush_weight"`
}

// healthStats provides a representation for output of
// `ceph -s -f json`.
type healthStats struct {
	PGMap struct {
		NumPGs     float64 `json:"num_pgs"`
		PGsByState []struct {
			Count  float64 `json:"count"`
			States string  `json:"state_name"`
		} `json:"pgs_by_state"`
	} `json:"pgmap"`
}
