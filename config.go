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

import "time"

// Option provides a safe way to update private
// variables of rebalancer before creating an
// instance of it.
type Option func(*Rebalancer)

// WithCephClient holds the ceph client connected
// to the ceph cluster we want to perform reweighting
// on.
func WithCephClient(val CephClient) Option {
	return func(r *Rebalancer) {
		r.ceph = val
	}
}

// WithMaxBackfillPGsAllowed allows changing the
// number of backfilling PGs that are acceptable
// to be ongoing while we issue another reweight
// operation.
func WithMaxBackfillPGsAllowed(val int) Option {
	return func(r *Rebalancer) {
		r.maxBackfillPGsAllowed = val
	}
}

// WithMaxRecoveryPGsAllowed allows changing the
// number of recovering PGs that are acceptable
// to be ongoing while we issue another reweight
// operation.
func WithMaxRecoveryPGsAllowed(val int) Option {
	return func(r *Rebalancer) {
		r.maxRecoveryPGsAllowed = val
	}
}

// WithTargetCrushWeightMap passes the mapping of each
// candidate OSD to its target CRUSH weight that it
// hopes to reach.
//
// This is a required option since we cannot run the
// reebalancer without any OSDs to reweight.
func WithTargetCrushWeightMap(val map[int]float64) Option {
	return func(r *Rebalancer) {
		r.targetCrushWeightMap = val
	}
}

// WithWeightIncrement updates the increment value by
// which each OSD will be upweighted.
func WithWeightIncrement(val float64) Option {
	return func(r *Rebalancer) {
		r.weightIncrement = val
	}
}

// WithSleepInterval updates the duration for which the
// rebalancer will sleep for between each of its reweight
// runs.
func WithSleepInterval(val time.Duration) Option {
	return func(r *Rebalancer) {
		r.sleepInterval = val
	}
}

// WithDryRun will change the mode of rebalancer. When
// dry-run is disabled, the reweights will be actually
// performed on the cluster.
//
// By default, dry-run is enabled to make sure no adverse
// impact occurs on the cluster until explicitly requested
// to.
func WithDryRun(val bool) Option {
	return func(r *Rebalancer) {
		r.dryRun = val
	}
}
