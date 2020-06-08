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

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	rebalancer "github.com/digitalocean/ceph-rebalancer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"
)

const (
	appName = "ceph-rebalancer"
)

func main() {
	app := cli.NewApp()
	app.Name = appName
	app.Authors = []*cli.Author{
		{
			Name:  "DigitalOcean Engineering",
			Email: "engineering@digitalocean.com",
		},
	}
	app.Usage = "Gradual data rebalancing tool for Ceph."
	app.Flags = []cli.Flag{
		cephUserFlag,
		cephConfigPathFlag,
		metricsAddrFlag,
	}
	app.Commands = commands

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

var commands = []*cli.Command{
	{
		Name:        "reweight",
		Usage:       "Reweight a set of OSDs",
		Description: "Reweight a set of OSDs",
		Flags: []cli.Flag{
			maxBackfillPGsFlag,
			maxRecoveryPGsFlag,
			targetOSDsCrushFlag,
			weightIncrementFlag,
			sleepDurationFlag,
			dryRunFlag,
		},
		Action: func(ctx *cli.Context) error {
			cc, err := rebalancer.NewCephClient(
				ctx.String(cephUserFlag.Name),
				ctx.String(cephConfigPathFlag.Name),
			)
			if err != nil {
				return fmt.Errorf("cannot create new ceph-client: %s", err)
			}
			defer cc.Close()

			twMap, err := parseTargetWeightMap(ctx.String(targetOSDsCrushFlag.Name))
			if err != nil {
				return fmt.Errorf("failed parsing target-weights: %s", err)
			}

			r, err := rebalancer.New(
				rebalancer.WithCephClient(cc),
				rebalancer.WithMaxBackfillPGsAllowed(ctx.Int(maxBackfillPGsFlag.Name)),
				rebalancer.WithMaxRecoveryPGsAllowed(ctx.Int(maxRecoveryPGsFlag.Name)),
				rebalancer.WithTargetCrushWeightMap(twMap),
				rebalancer.WithWeightIncrement(ctx.Float64(weightIncrementFlag.Name)),
				rebalancer.WithSleepInterval(ctx.Duration(sleepDurationFlag.Name)),
				rebalancer.WithDryRun(ctx.Bool(dryRunFlag.Name)),
			)
			if err != nil {
				return fmt.Errorf("initializing rebalancer failed: %s", err)
			}

			go func() {
				prometheus.MustRegister(r)
				http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					w.Write(
						[]byte(`
							<html>
								<head><title>Ceph-Rebalancer</title></head>
								<body>
									<h1>Prometheus metrics for Ceph Rebalancer</h1>
									<p><a href='/metrics'>Metrics</a></p>
								</body>
							</html>
						`),
					)
				})
				http.Handle("/metrics", promhttp.Handler())

				metricsAddr := ctx.String(metricsAddrFlag.Name)
				if err := http.ListenAndServe(metricsAddr, nil); err != nil {
					log.Fatalf("cannot start metrics server on %q: %s", metricsAddr, err)
				}
			}()

			cctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			r.Run(cctx)
			return nil
		},
	},
}

// The target-weight map is expected in the following csv format:
//  '1:2.5999,2:2.5999,3:4.798'
//
// This will be broken down into the following map:
//  map[int]float64{
//	   1: 2.5999,
//	   2: 2.5999,
//	   3: 4.798,
//  }
// when no errors are found in the input.
func parseTargetWeightMap(twStr string) (map[int]float64, error) {
	parts := strings.Split(twStr, ",")
	if len(parts) == 0 {
		return nil, errors.New("empty target-weight map found")
	}

	twMap := make(map[int]float64, len(parts))
	for _, part := range parts {
		osdAndWeight := strings.SplitN(part, ":", 2)
		if len(osdAndWeight) < 2 {
			return nil, fmt.Errorf("incorrect osd-weight pair provided: %q", part)
		}

		osdID := osdAndWeight[0]
		o, err := strconv.Atoi(osdID)
		if err != nil {
			return nil, fmt.Errorf("osd id should be an integer, %q provided: %s", osdID, err)
		}

		weight := osdAndWeight[1]
		w, err := strconv.ParseFloat(weight, 64)
		if err != nil {
			return nil, fmt.Errorf("weight should be a float, %q provided: %s", weight, err)
		}

		twMap[o] = w
	}

	return twMap, nil
}

var (
	cephUserFlag = &cli.StringFlag{
		Name:  "ceph-user",
		Usage: "Ceph username provided without the 'client.' prefix.",
	}

	cephConfigPathFlag = &cli.StringFlag{
		Name:  "ceph-conf",
		Value: "/etc/ceph/ceph.conf",
		Usage: "Ceph config used for establishing connection to the cluster.",
	}

	metricsAddrFlag = &cli.StringFlag{
		Name:  "metrics-addr",
		Value: ":8928",
		Usage: "Address on which metrics will be exported. Needs exposed in Docker.release too.",
	}
)

var (
	maxBackfillPGsFlag = &cli.IntFlag{
		Name:  "max-backfill-pgs",
		Value: 10,
		Usage: "Number of maximum PGs allowed to be in backfill/backfill_wait state.",
	}

	maxRecoveryPGsFlag = &cli.IntFlag{
		Name:  "max-recovery-pgs",
		Value: 10,
		Usage: "Number of maximum PGs allowed to be in recovering/recovery_wait state.",
	}

	targetOSDsCrushFlag = &cli.StringFlag{
		Name:  "target-osd-crush-weights",
		Value: "",
		Usage: "OSDs and CRUSH weights provided in format of: 'osd-id:weight,osd-id:weight'.",
	}

	weightIncrementFlag = &cli.Float64Flag{
		Name:  "weight-increment",
		Value: 0.02,
		Usage: "Value by which the CRUSH weights will be incremented per iteration.",
	}

	sleepDurationFlag = &cli.DurationFlag{
		Name:  "sleep-duration",
		Value: 5 * time.Minute,
		Usage: "The amount of time to sleep between each iteration of reweight run.",
	}

	dryRunFlag = &cli.BoolFlag{
		Name:  "dry-run",
		Value: true,
		Usage: "No action taken on the cluster when true. Explicitly pass as false for rebalance to take place.",
	}
)
