# Disk Cache Quickstart Guide [![Slack](https://slack.minio.io/slack?type=svg)](https://slack.minio.io)

Disk caching feature here refers to the use of caching disks to store content closer to the tenants. For instance, if you access an object from a lets say `gateway azure` setup and download the object that gets cached, each subsequent request on the object gets served directly from the cache drives until it expires. This feature allows Minio users to have

- Object to be delivered with the best possible performance.
- Dramatic improvements for time to first byte for any object.

## Get started

### 1. Prerequisites
Install Minio - [Minio Quickstart Guide](https://docs.minio.io/docs/minio-quickstart-guide).

### 2. Run Minio with cache
Disk caching can be enabled by updating the `cache` config settings for Minio server. Config `cache` settings takes the mounted drive(s) or directory paths, cache expiry duration (in days) and any wildcard patterns to exclude from being cached.

```json
"cache": {
	"drives": ["/mnt/drive1", "/mnt/drive2", "/mnt/drive3"],
	"expiry": 90,
	"exclude": ["*.pdf","mybucket/*"],
	"maxuse" : 70,
},
```

To update the configuration, use `mc admin config get` command to get the current configuration file for the minio cluster in json format, and save it locally.
```sh
$ mc admin config get myminio/ > /tmp/myconfig
```
After updating the cache configuration in /tmp/myconfig , use `mc admin config set` command to update the configuration for the cluster.Restart the Minio server to put the changes into effect.
```sh
$ mc admin config set myminio < /tmp/myconfig
```
The cache settings may also be set through environment variables. When set, environment variables override any `cache` config settings for Minio server. Following example uses `/mnt/drive1`, `/mnt/drive2` ,`/mnt/cache1` ... `/mnt/cache3` for caching, with expiry up to 90 days while excluding all objects under bucket `mybucket` and all objects with '.pdf' as extension while starting a standalone erasure coded setup. Cache max usage is restricted to 80% of disk capacity in this example.

```bash
export MINIO_CACHE_DRIVES="/mnt/drive1;/mnt/drive2;/mnt/cache{1...3}"
export MINIO_CACHE_EXPIRY=90
export MINIO_CACHE_EXCLUDE="*.pdf;mybucket/*"
export MINIO_CACHE_MAXUSE=80
minio server /export{1...24}
```

### 3. Test your setup
To test this setup, access the Minio server via browser or [`mc`](https://docs.minio.io/docs/minio-client-quickstart-guide). You’ll see the uploaded files are accessible from the all the Minio endpoints.

# Explore Further
- [Disk cache design](https://github.com/minio/minio/blob/master/docs/disk-caching/DESIGN.md)
- [Use `mc` with Minio Server](https://docs.minio.io/docs/minio-client-quickstart-guide)
- [Use `aws-cli` with Minio Server](https://docs.minio.io/docs/aws-cli-with-minio)
- [Use `s3cmd` with Minio Server](https://docs.minio.io/docs/s3cmd-with-minio)
- [Use `minio-go` SDK with Minio Server](https://docs.minio.io/docs/golang-client-quickstart-guide)
- [The Minio documentation website](https://docs.minio.io)
