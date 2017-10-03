# Introduction

This repository provides the go-based implementation of Pydio. It is a full rewrite of Pydio backends and is built on the
following foundations : 

- Architecture is fully micro-service oriented, leveraging the [Micro](https://github.com/micro/micro) framework.
- It creates a storage backend including file storage, metadata, ACL and views.
- Main access API's are based on S3 api thanks to the [Minio](https://github.com/minio/minio) server, used both at the 
deepest level (object storage) and highest level (API).
- Inter-services communication is done using gRPC protocol for best performances, services are described using Protobuf.

A couple of pre-requisites are necessary: 

- **Consul**: [https://consul.io](https://consul.io) by Hashicorp (or via Docker). It is the service registry for automatic discovery.
- Modified version of **Protoc** as provided by the [Micro forked version](https://github.com/micro/protobuf) (go get and compile to replace original Grpc Protoc)
- For the moment, actual backends for services data (except files) are based on **MySQL**. By default, configs expected a "micro" 
database to be present, and will connect with "root" user, without password, and privilege to create tables.

# Services

## Minimal Set

In order to run, you have to first run at least the following services: 

### services/configs

This bootloader will load some basic configs from `services/configs/file/sample.json` and insert them in the DB. Then each service can 
query the pydio.service.config to get it's own configuration.

`$ go run main.go --config_file=file/sample.json`

### services/datasource

This is the core of the storage, and it is in fact composed of 3 services : 

- Minio : an instance of a minio server, providing access to a target folder.
- Index : A db-based matrix-encoded fast indexer providing the tree of nodes for this datasource
- Sync  : A listener on Minio s3 events that keeps the index in sync. 

The main idea is that every read operation are done from the index, and every write operation are performed directly 
on the Minio server, then synced to the index.

To launch all 3 services, use 

`$ go run objects/main.go --folder=/path/to/folder/to/serve --datasource=miniods1`  
`$ go run index/main.go --datasource=miniods1`  
`$ go run sync/main.go --datasource=miniods1 [--watch=/path/to/folder/to/watch] [--normalize=true]`  

Optionally : 

- On Mac, add `--normalize=true` to make sure that utf-8 strings are NFC-encoded
- If the folder to server is to be modified from outside pydio, use `--sync="/path/to/folder/to/server"` to listen on the 
FS events instead of the S3 events provided by Minio.
- You can start each service separately.

### services/meta

A simple meta store for all nodes metadata, currently based on MySQL. No fancy options here.

`$ go run main.go`

### services/tree

Aggregator service will merge all data from various datasources and metadata service into one unique tree. Currently the first 
level of folders will reflect the datasources started.

## Good to have

For debugging and tracing request all over the application, we provide easy to use docker-compose to start a trace aggregator 
& visualisation service, and a logs aggregator based on ELK stack.

### Logs

Under services/backend/monitor,  
`$ docker-compose up`  
Then access Kibana on [http://127.0.0.1:5601/](http://127.0.0.1:5601/)

### Traces

Under services/backend/trace
`$ docker-compose up`  
Then access Jaeger UI on [http://127.0.0.1:16686/](http://127.0.0.1:16686/)

## Work in progress

### services/acl

Add ACL on top of the tree

### services/search

Hooked to the Meta service events, this provides a simple full-text search on file names using Bleve. To be extended to support
ElasticSearch as well.

### services/workers

Listing to the tree changes, this provide a pool-based task service for extracting image metadata (dimensions, exif if possible) and
generating Thumbnails in pure-go. For that service, make sure that your datasource folder has a ./thumbs sibling folder that will be used
as a store for all generated thumbnails.

# Writing new services

New services must comply to the following pattern

 - Main method is just starting the service, put the service initialization code in `yourservicename/service/service.go`
 - Use the `common/templates/service.go` methods to create new MicroServices without hassle.
 - Use logging.FromContext() instead of log.Print... 
 - Protobuf handler is must be implemented separately
 - Actual logic must be declared using interfaces and have at least two implementation (DAO) : stub and mysql.
 - Every file must provide tests, using **GoConvey** library.
 
Tips & tricks

 - Once protoc is compiled from the Micro fork, use  
 `$ protoc --go_out=plugins=micro:. yourfile.proto`

 - When there are dependencies, use -I to load other files, example :
 `$ protoc -I/path/to/protobuf/src/google/protobuf -I$GO_PATH/src -I./idm --go_out=plugins=micro:idm/ idm/idm.proto `