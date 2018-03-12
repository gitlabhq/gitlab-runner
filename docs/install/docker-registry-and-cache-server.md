# Prepare the Docker Registry and Cache Server

To speedup the builds we advise to setup a personal Docker registry server
working in proxy mode. A cache server is also recommended.

## Install Docker Registry

>**Note:**
Read more in [Distributed registry mirroring][registry].

1. Login to a dedicated machine where Docker registry proxy will be running
2. Make sure that Docker Engine is installed on this machine
3. Create a new Docker registry:

    ```bash
    docker run -d -p 6000:5000 \
        -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
        --restart always \
        --name registry registry:2
    ```

    You can modify the port number (`6000`) to expose Docker registry on a
    different port.

4. Check the IP address of the server:

    ```bash
    hostname --ip-address
    ```

    You should preferably choose the private networking IP address. The private
    networking is usually the fastest solution for internal communication
    between machines of a single provider (DigitalOcean, AWS, Azure, etc,)
    Usually the private networking is also not accounted to your monthly
    bandwidth limit.

5. Docker registry will be accessible under `MY_REGISTRY_IP:6000`.

## Install the cache server

>**Note:**
You can use any other S3-compatible server, including [Amazon S3][S3]. Read
more in [Distributed runners caching][caching].

1. Login to a dedicated machine where the cache server will be running
1. Make sure that Docker Engine is installed on that machine
1. Start [minio], a simple S3-compatible server written in Go:

    ```bash
    docker run -it --restart always -p 9005:9000 \
            -v /.minio:/root/.minio -v /export:/export \
            --name minio \
            minio/minio:latest server /export
    ```

    You can modify the port `9005` to expose the cache server on different port.

1. Check the IP address of the server:

    ```bash
    hostname --ip-address
    ```

1. Your cache server will be available at `MY_CACHE_IP:9005`
1. Read the Access and Secret Key of minio with: `sudo cat /.minio/config.json`
1. Create a bucket that will be used by the Runner: `sudo mkdir /export/runner`.
   `runner` is the name of the bucket in that case. If you choose a different
   bucket then it will be different
1. All caches will be stored in the `/export` directory

[caching]: ../configuration/autoscale.md#distributed-runners-caching
[registry]: ../configuration/autoscale.md#distributed-docker-registry-mirroring
