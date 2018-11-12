# Multi-tenant Minio Deployment Guide [![Slack](https://slack.minio.io/slack?type=svg)](https://slack.minio.io) [![Go Report Card](https://goreportcard.com/badge/pydio/minio-srv)](https://goreportcard.com/report/pydio/minio-srv) [![Docker Pulls](https://img.shields.io/docker/pulls/pydio/minio-srv.svg?maxAge=604800)](https://hub.docker.com/r/pydio/minio-srv/) [![codecov](https://codecov.io/gh/pydio/minio-srv/branch/master/graph/badge.svg)](https://codecov.io/gh/pydio/minio-srv)

## Standalone Deployment
To host multiple tenants on a single machine, run one Minio server per tenant with dedicated HTTPS port, config and data directory.  

#### Example 1 : Single host, single drive

This example hosts 3 tenants on a single drive.
```sh
minio --config-dir ~/tenant1 server --address :9001 /data/tenant1
minio --config-dir ~/tenant2 server --address :9002 /data/tenant2
minio --config-dir ~/tenant3 server --address :9003 /data/tenant3
```

![Example-1](https://github.com/pydio/minio-srv/blob/master/docs/screenshots/Example-1.jpg?raw=true)

#### Example 2 : Single host, multiple drives (erasure code)

This example hosts 3 tenants on multiple drives.
```sh
minio --config-dir ~/tenant1 server --address :9001 /disk1/data/tenant1 /disk2/data/tenant1 /disk3/data/tenant1 /disk4/data/tenant1
minio --config-dir ~/tenant2 server --address :9002 /disk1/data/tenant2 /disk2/data/tenant2 /disk3/data/tenant2 /disk4/data/tenant2
minio --config-dir ~/tenant3 server --address :9003 /disk1/data/tenant3 /disk2/data/tenant3 /disk3/data/tenant3 /disk4/data/tenant3
```
![Example-2](https://github.com/pydio/minio-srv/blob/master/docs/screenshots/Example-2.jpg?raw=true)

## Distributed Deployment
To host multiple tenants in a distributed environment, run several distributed Minio instances concurrently.  

#### Example 1 : Multiple host, multiple drives (erasure code)

This example hosts 3 tenants on a 4 node distributed setup. Execute the following command on all the four nodes. 

```sh
export MINIO_ACCESS_KEY=<TENANT1_ACCESS_KEY>
export MINIO_SECRET_KEY=<TENANT1_SECRET_KEY>
minio --config-dir ~/tenant1 server --address :9001 http://192.168.10.11/data/tenant1 http://192.168.10.12/data/tenant1 http://192.168.10.13/data/tenant1 http://192.168.10.14/data/tenant1

export MINIO_ACCESS_KEY=<TENANT2_ACCESS_KEY>
export MINIO_SECRET_KEY=<TENANT2_SECRET_KEY>
minio --config-dir ~/tenant2 server --address :9002 http://192.168.10.11/data/tenant2 http://192.168.10.12/data/tenant2 http://192.168.10.13/data/tenant2 http://192.168.10.14/data/tenant2

export MINIO_ACCESS_KEY=<TENANT3_ACCESS_KEY>
export MINIO_SECRET_KEY=<TENANT3_SECRET_KEY>
minio --config-dir ~/tenant3 server --address :9003 http://192.168.10.11/data/tenant3 http://192.168.10.12/data/tenant3 http://192.168.10.13/data/tenant3 http://192.168.10.14/data/tenant3
```

![Example-3](https://github.com/pydio/minio-srv/blob/master/docs/screenshots/Example-3.jpg?raw=true)

*Note*

Please remember that defining and exporting credential environment variables, `MINIO_ACCESS_KEY` and  `MINIO_SECRET_KEY`, is **mandatory** on distributed deployments. It is also required to define and export domain environment variable, `MINIO_DOMAIN`, if domain information is needed to be set.

## Cloud Scale Deployment

For large scale multi-tenant Minio deployments, we recommend using one of the popular container orchestration platforms, e.g. Kubernetes, DC/OS or Docker Swarm. Refer [this document](https://docs.minio.io/docs/minio-deployment-quickstart-guide) to get started with Minio on orchestration platforms.  


