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

package rebalancer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoReweight(t *testing.T) {
	for _, tt := range []struct {
		name string

		weightIncrement float64
		backfillingPGs  int
		recoveringPGs   int
		osdTree         *OSDTreeOut
		dryRun          bool

		iterations      int
		reweightCount   int
		crushWeightMap  map[int]float64
		targetWeightMap map[int]float64
	}{
		{
			name: "High BackfillPGs",

			backfillingPGs: 100,
			osdTree: &OSDTreeOut{
				Nodes: nil,
			},
			reweightCount:  0,
			crushWeightMap: nil,
			targetWeightMap: map[int]float64{
				1: 7.4999,
				2: 15.4999,
			},
		},
		{
			name: "High RecoveryPGs",

			recoveringPGs: 100,
			osdTree: &OSDTreeOut{
				Nodes: nil,
			},
			reweightCount:  0,
			crushWeightMap: nil,
			targetWeightMap: map[int]float64{
				1: 7.4999,
				2: 15.4999,
			},
		},
		{
			name: "Zero Increment",

			osdTree: &OSDTreeOut{
				Nodes: nil,
			},
			reweightCount:  0,
			crushWeightMap: nil,

			weightIncrement: 0,
			targetWeightMap: map[int]float64{
				1: 7.4999,
				2: 15.4999,
			},
		},
		{
			name: "DryRun Enabled",

			dryRun: true,
			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount:  0,
			crushWeightMap: nil,

			weightIncrement: 4.0,
			targetWeightMap: map[int]float64{
				1: 7.4999,
				2: 15.4999,
			},
		},
		{
			name: "Single Increment",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 1,
			crushWeightMap: map[int]float64{
				1: 4.0,
				2: 4.0,
			},

			weightIncrement: 4.0,
			iterations:      1,
			targetWeightMap: map[int]float64{
				1: 4.0,
				2: 4.0,
			},
		},
		{
			name: "Distinct TargetWeights",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 1,
			crushWeightMap: map[int]float64{
				1: 4.0,
				2: 4.0,
			},

			weightIncrement: 4.0,
			iterations:      1,
			targetWeightMap: map[int]float64{
				1: 16.0,
				2: 8.0,
			},
		},
		{
			name: "Same TargetWeight Reached",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 1,
			crushWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},

			weightIncrement: 4.0, // Increment is x2 times the target weight!
			iterations:      10,
			targetWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},
		},
		{
			name: "Distinct TargetWeight Reached",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 1,
			crushWeightMap: map[int]float64{
				1: 2.0,
				2: 4.0,
			},

			weightIncrement: 8.0, // Increment is x2 times the largest target weight!
			iterations:      10,
			targetWeightMap: map[int]float64{
				1: 2.0,
				2: 4.0,
			},
		},
		{
			name: "Granular TargetWeight Reached",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 125,
			crushWeightMap: map[int]float64{
				1: 2.4999,
				2: 2.4999,
			},

			weightIncrement: 0.02, // tiny increment!
			iterations:      1000,
			targetWeightMap: map[int]float64{
				1: 2.4999,
				2: 2.4999,
			},
		},
		{
			name: "Non-Zero CrushWeight TargetWeight Reached",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 1.0, // Non-zero Crush weight.
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 1,
			crushWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},

			weightIncrement: 4.0, // Increment is x2 times the target weight!
			iterations:      10,
			targetWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},
		},
		{
			name: "Same TargetWeight Small Iterations",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 10,
			crushWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},

			weightIncrement: 0.2,
			iterations:      10,
			targetWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},
		},
		{
			name: "Incomplete Iterations",

			osdTree: &OSDTreeOut{
				Nodes: []nodeType{
					{
						ID:          1,
						Type:        "osd",
						CrushWeight: 0,
					},
					{
						ID:          2,
						Type:        "osd",
						CrushWeight: 0,
					},
				},
			},
			reweightCount: 5,
			crushWeightMap: map[int]float64{
				1: 1.0, // We can only partially fill these, sadly!
				2: 1.0,
			},

			weightIncrement: 0.2,
			iterations:      5, // No. of iterations is less than what is needed to fill up OSDs.
			targetWeightMap: map[int]float64{
				1: 2.0,
				2: 2.0,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tc := &testCephClient{
				backfillingPGs: tt.backfillingPGs,
				recoveringPGs:  tt.recoveringPGs,
				osdTree:        tt.osdTree,
			}
			defer tc.Close()

			r, err := New(
				WithCephClient(tc),
				WithWeightIncrement(tt.weightIncrement),
				WithTargetCrushWeightMap(tt.targetWeightMap),
				WithDryRun(tt.dryRun),
			)
			if err != nil {
				t.Fatalf("failed initializing rebalancer")
			}

			// Perform reweights.
			if tt.iterations == 0 {
				tt.iterations = 1
			}
			for i := tt.iterations; i > 0; i-- {
				r.DoReweight()
			}

			assert.Equal(t,
				tt.reweightCount*len(tt.osdTree.Nodes), tc.reweightCount, "reweight counts should match")

			assert.Equal(t,
				tt.crushWeightMap, tc.crushWeightMap, "crush weight changes should match")
		})
	}
}

var _ CephClient = &testCephClient{}

type testCephClient struct {
	reweightCount  int
	crushWeightMap map[int]float64

	osdTree        *OSDTreeOut
	backfillingPGs int
	recoveringPGs  int
}

func (c *testCephClient) BackfillingPGs() (int, error) {
	return c.backfillingPGs, nil
}

func (c *testCephClient) RecoveringPGs() (int, error) {
	return c.recoveringPGs, nil
}

func (c *testCephClient) OSDTree() (*OSDTreeOut, error) {
	return c.osdTree, nil
}

func (c *testCephClient) CrushReweight(osdID int, crushWeight float64) error {
	for i := range c.osdTree.Nodes {
		if c.osdTree.Nodes[i].ID == osdID {
			c.osdTree.Nodes[i].CrushWeight = crushWeight
			break
		}
	}

	if c.crushWeightMap == nil {
		c.crushWeightMap = map[int]float64{}
	}
	c.crushWeightMap[osdID] = crushWeight
	c.reweightCount++
	return nil
}

func (c *testCephClient) Close() {
	return
}
