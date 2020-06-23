# archimedes
![Build](https://github.com/digitalocean/archimedes/workflows/Build/badge.svg?branch=master) ![License](https://github.com/digitalocean/archimedes/workflows/License/badge.svg?branch=master) [![Apache License](https://img.shields.io/hexpm/l/plug)](LICENSE)

Automatic and gradual rebalancing mechanism for Ceph OSDs. This process is designed to be deployed and run as a docker container that periodically reweights given set of OSDs to their target weights. It does across multiple iterations where each iteration upweights an OSD by `--weight-increment` value. The reweights are applied to CRUSH reweight parameter of an OSD and not the OSD reweight parameter.

## Usage

This mechanism is designed to run as a docker container in the background. We have to build the image from the provided Dockerfile before we use it.

```
docker build -t docker.digitalocean.com/archimedes:latest -f Dockerfile.release .
```

You will want to change the docker image name/endpoint based on your setup. Once the image is built successfully, you can run `docker push <image>:tag` for pushing the image to its repository assuming you want save it for later or use it quickly from other machines in your ensemble.

The reweight run is initiated with the following command:

```
docker run --rm -v /etc/ceph:/etc/ceph -it docker.digitalocean.com/archimedes:latest --ceph-user admin reweight --target-osd-crush-weights "1:1.4999,2:1.4999,3:7.7999" --weight-increment 0.02
```

It is expected that `/etc/ceph` directory on the host in the above case contains both:
* The user keyring, which will be `ceph.client.admin.keyring` since we passed in user as `admin`.
* The ceph config for talking to the cluster: `ceph.conf`.

Once the container resolves the connection to the cluster correctly, it will run in background until the target weight for every single OSD, until the last one, is achieved.

The runs are further customizable. We can control options like the number of PGs we should expect backfilling / recovering until we kick off next iteration of reweights, etc. The list of options should pop up on `--help`.

```
docker run --rm -it docker.digitalocean.com/archimedes:latest reweight --help
```

## Metrics and Logging

Our code uses `logrus` for structured logging which should be visible via docker logs.

```
docker logs -f docker.digitalocean.com/archimedes:latest
```

It also exposes metrics to be scraped by prometheus exporter at `:8928` by default. This port address can be changed by passing in `--metrics-addr` to make it listen elsewhere. We should be able to see the exported metrics at the following endpoint.

```
curl http://localhost:8928/metrics
```

## Development

The code is written in Golang and compatibility is tested with v1.13+ runtimes.

There is a helper Makefile included to assist with needs of testing. Running the `test` target should build and run the slew of tests to make sure our new changes are safe.

```
make test
```
