# Minio Deployment Quickstart Guide [![Slack](https://slack.minio.io/slack?type=svg)](https://slack.minio.io) [![Go Report Card](https://goreportcard.com/badge/pydio/minio-srv)](https://goreportcard.com/report/pydio/minio-srv) [![Docker Pulls](https://img.shields.io/docker/pulls/pydio/minio-srv.svg?maxAge=604800)](https://hub.docker.com/r/pydio/minio-srv/) [![codecov](https://codecov.io/gh/pydio/minio-srv/branch/master/graph/badge.svg)](https://codecov.io/gh/pydio/minio-srv)

Minio is a cloud-native application designed to scale in a sustainable manner in multi-tenant environments. Orchestration platforms provide perfect launchpad for Minio to scale. Below is the list of Minio deployment documents for various orchestration platforms:

| Orchestration platforms|
|:---|
| [`Docker Swarm`](https://docs.minio.io/docs/deploy-minio-on-docker-swarm) |
| [`Docker Compose`](https://docs.minio.io/docs/deploy-minio-on-docker-compose) |
| [`Kubernetes`](https://docs.minio.io/docs/deploy-minio-on-kubernetes) |
| [`DC/OS`](https://docs.minio.io/docs/deploy-minio-on-dc-os) |

## Why is Minio cloud-native?
The term cloud-native revolves around the idea of applications deployed as micro services, that scale well. It is not about just retrofitting monolithic applications onto modern container based compute environment. A cloud-native application is portable and resilient by design, and can scale horizontally by simply replicating. Modern orchestration platforms like Swarm, Kubernetes and DC/OS make replicating and managing containers in huge clusters easier than ever.

While containers provide isolated application execution environment, orchestration platforms allow seamless scaling by helping replicate and manage containers. Minio extends this by adding isolated storage environment for each tenant.

Minio is built ground up on the cloud-native premise. With features like erasure-coding, distributed and shared setup, it focuses only on storage and does it very well. While, it can be scaled by just replicating Minio instances per tenant via an orchestration platform.  

> In a cloud-native environment, scalability is not a function of the application but the orchestration platform.

In a typical modern infrastructure deployment, application, database, key-store, etc. already live in containers and are managed by orchestration platforms. Minio brings robust, scalable, AWS S3 compatible object storage to the lot.

![Cloud-native](https://github.com/pydio/minio-srv/blob/master/docs/screenshots/Minio_Cloud_Native_Arch.jpg?raw=true)
