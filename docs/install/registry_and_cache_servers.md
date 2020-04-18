# Install your own container registry and cache server

When using the Runner in [autoscale mode](../configuration/autoscale.md), it
is advised to set up a personal container registry and a cache server.

## Install a proxy container registry

1. Login to a dedicated machine where the container registry proxy will be running
1. Make sure that [Docker Engine](https://docs.docker.com/install/) is installed
   on that machine
1. Create a new container registry:

   ```shell
   docker run -d -p 6000:5000 \
       -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
       --restart always \
       --name registry registry:2
   ```

   You can modify the port number (`6000`) to expose the registry on a
   different port

1. Check the IP address of the server:

   ```shell
   hostname --ip-address
   ```

   You should preferably choose the private networking IP address. The private
   networking is usually the fastest solution for internal communication
   between machines of a single provider (DigitalOcean, AWS, Azure, etc)
   Usually the private networking is also not accounted to your monthly
   bandwidth limit.

1. Docker registry will be accessible under `MY_REGISTRY_IP:6000`

You can now proceed and
[configure `config.toml`](../configuration/autoscale.md#distributed-container-registry-mirroring)
to use the new registry server.

## Install your own cache server

If you don't want to use a SaaS S3 server, you can install your own
S3-compatible caching server:

1. Login to a dedicated machine where the cache server will be running
1. Make sure that [Docker Engine](https://docs.docker.com/install/) is installed
   on that machine
1. Start [MinIO](https://min.io), a simple S3-compatible server written in Go:

   ```shell
   docker run -it --restart always -p 9005:9000 \
           -v /.minio:/root/.minio -v /export:/export \
           --name minio \
           minio/minio:latest server /export
   ```

   You can modify the port `9005` to expose the cache server on different port

1. Check the IP address of the server:

   ```shell
   hostname --ip-address
   ```

1. Your cache server will be available at `MY_CACHE_IP:9005`
1. Create a bucket that will be used by the Runner:

   ```shell
   sudo mkdir /export/runner
   ```

   `runner` is the name of the bucket in that case. If you choose a different
   bucket, then it will be different. All caches will be stored in the
   `/export` directory.

1. Read the Access and Secret Key of MinIO and use it to configure the Runner:

   ```shell
   sudo cat /export/.minio.sys/config/config.json | grep Key
   ```

You can now proceed and
[configure `config.toml`](../configuration/autoscale.md#distributed-runners-caching)
to use the new cache server.
