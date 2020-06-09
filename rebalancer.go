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
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

const (
	serviceName = "rebalancer"
)
const (
	roundToPlaces = 4
)

// Rebalancer is responsible for performing data rebalancing
// by control weight changes to OSDs.
type Rebalancer struct {
	ceph CephClient

	maxBackfillPGsAllowed int
	maxRecoveryPGsAllowed int

	targetCrushWeightMap map[int]float64
	weightIncrement      float64

	sleepInterval time.Duration
	dryRun        bool

	crushWeightMap  map[int]float64
	crushWeightDesc *prometheus.Desc
	targetOSDsDesc  *prometheus.Desc
}

// New returns a new instance of Rebalancer. It is expected
// that non-empty values for map of osd<->crush weights
// is passed as an input.
func New(opt ...Option) (*Rebalancer, error) {
	r := &Rebalancer{
		maxBackfillPGsAllowed: 10,
		maxRecoveryPGsAllowed: 10,
		weightIncrement:       0.02,
		sleepInterval:         30 * time.Second,
		dryRun:                true,

		crushWeightMap: map[int]float64{},
		crushWeightDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_crushweight", serviceName),
			"Crush Weight set for a given OSD",
			[]string{
				"osd",
			}, nil,
		),
		targetOSDsDesc: prometheus.NewDesc(
			fmt.Sprintf("%s_target_osds_total", serviceName),
			"Count of target OSDs still left to be upweighted",
			nil, nil,
		),
	}

	for _, fn := range opt {
		fn(r)
	}

	if len(r.targetCrushWeightMap) == 0 {
		return nil, errors.New("no weight map found")
	}

	// A ceph client with an existing connection to the cluster
	// is expected as an input. It is also the caller's responsibility
	// to Close() the connection that's established for the ceph client.
	if r.ceph == nil {
		return nil, errors.New("no ceph client found")
	}

	return r, nil
}

// Run performs continues reweighting by pausing for
// `sleepInterval` duration between runs. It returns
// when either the caller context is cancelled or
// when all entries from osd<->target-crush-weight
// are processed.
func (r *Rebalancer) Run(ctx context.Context) {
	ticker := time.NewTicker(r.sleepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(r.targetCrushWeightMap) <= 0 {
				log.Info("all given osds completed reweighting")
				return
			}

			r.DoReweight()
		}
	}
}

// DoReweight is the main function where the validation and
// actual crush reweighting occurs.
func (r *Rebalancer) DoReweight() {
	bpgs, err := r.ceph.BackfillingPGs()
	if err != nil {
		log.WithError(err).Error("failed checking for backfilling pgs")
		return
	}
	if bpgs > r.maxBackfillPGsAllowed {
		log.WithField("backfill.pgs", bpgs).Warn("skipping reweighting, backfilling pgs found")
		return
	}

	rpgs, err := r.ceph.RecoveringPGs()
	if err != nil {
		log.WithError(err).Error("failed checking for recovering pgs")
		return
	}
	if rpgs > r.maxRecoveryPGsAllowed {
		log.WithField("recovery.pgs", rpgs).Warn("skipping reweighting, recovering pgs found")
		return
	}

	cws := r.extractCurrentWeights()
	for osd, tw := range r.targetCrushWeightMap {
		ll := log.WithField("osd", osd)

		cw, ok := cws[osd]
		if !ok {
			ll.Error("cannot find osd in current osd tree")

			delete(r.targetCrushWeightMap, osd)
			continue
		}

		ll = ll.WithField("target.weight", tw).WithField("current.weight", cw)
		if cw >= tw {
			// target weight achieved
			ll.Info("target weight achieved")

			delete(r.targetCrushWeightMap, osd)
			continue
		}

		// If the increment takes our new weight larger than target-weight, then
		// we resort to setting the target weight instead. The `roundToPlaces` hack
		// is required to make sure we hit the target-weight much more accurately
		// and don't finish when we are 0.00001 away from it.
		tenExp := math.Pow10(roundToPlaces)
		weight := math.Min(((cw+r.weightIncrement)*tenExp)/tenExp, tw)

		ll = ll.WithField("weight", weight).WithField("inc", r.weightIncrement)
		if weight <= 0 {
			ll.Error("0 or negative weight found")

			delete(r.targetCrushWeightMap, osd)
			continue
		}

		// If the next reweight value is the same one we set previously, that
		// means we have achieved optimal weight. Nothing more to do here.
		if w, ok := r.crushWeightMap[osd]; ok {
			if w == weight {
				ll.Info("optimal weight achieved!")

				delete(r.targetCrushWeightMap, osd)
				continue
			}
		}

		if r.dryRun {
			ll.Info("weight will be applied in the actual run")

			delete(r.targetCrushWeightMap, osd)
			continue
		}

		if err := r.doReweight(osd, weight); err != nil {
			ll.WithError(err).Error("cannot reweight osd")
			continue
		}

		ll.Info("reweight applied!")
	}
}

func (r *Rebalancer) extractCurrentWeights() map[int]float64 {
	out, err := r.ceph.OSDTree()
	if err != nil {
		log.WithError(err).Error("failed to get output of osd-tree")
		return nil
	}

	osdsToReweight := make(map[int]float64)
	for _, node := range out.Nodes {
		if node.Type != "osd" {
			continue
		}

		_, ok := r.targetCrushWeightMap[node.ID]
		if ok {
			osdsToReweight[node.ID] = float64(node.CrushWeight)
		}
	}

	return osdsToReweight
}

func (r *Rebalancer) doReweight(osdID int, crushWeight float64) error {
	r.crushWeightMap[osdID] = crushWeight
	return r.ceph.CrushReweight(osdID, crushWeight)
}

// Verify that Rebalancer implements prometheus.Collector.
var _ prometheus.Collector = &Rebalancer{}

// Collect is responsible for collecting values for all declared
// metrics.
func (r *Rebalancer) Collect(ch chan<- prometheus.Metric) {
	for osd, cw := range r.crushWeightMap {
		ch <- prometheus.MustNewConstMetric(
			r.crushWeightDesc,
			prometheus.GaugeValue,
			float64(cw),
			strconv.Itoa(osd),
		)
	}
	ch <- prometheus.MustNewConstMetric(
		r.targetOSDsDesc,
		prometheus.GaugeValue,
		float64(len(r.targetCrushWeightMap)),
	)
}

// Describe returns the descriptions for registered metrics.
func (r *Rebalancer) Describe(ch chan<- *prometheus.Desc) {
	ch <- r.crushWeightDesc
	ch <- r.targetOSDsDesc
}
