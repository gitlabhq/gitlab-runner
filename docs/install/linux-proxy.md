### Running the runner behind a proxy

#### Assumtions
* you've already installed docker and the gitlab runner.
* you are using `cntlm` as a local proxy.  

The last point has **2** major advantages compared to adding the proxy details everywhere directly.  

* one single source where you need to change your credentials  
* the credentials can not be accessed from the docker runners  

#### Configuring cntlm
The config file for cntlm is `/etc/cntlm.conf`  
Enter your username, password, domain and proxy hosts there.  
Also make sure to remove the leading # from this line:  
`Gateway yes`  
**This is important! If you don't do that, your docker containers won't be able to use your local CNTLM proxy!**
After you changed your config, restart CNTLM with:  
`service cntlm restart`

#### Proxy for Docker image download  
Refer to https://docs.docker.com/engine/admin/systemd/#http-proxy  

1. Create a systemd drop-in directory for the docker service:  
`mkdir /etc/systemd/system/docker.service.d`  
2. Create a file called `/etc/systemd/system/docker.service.d/http-proxy.conf` that adds the `HTTP_PROXY` environment variable:  

```
[Service]
Environment="HTTP_PROXY=http://localhost:3128/"
Environment="HTTPS_PROXY=http://localhost:3128/"
```  

3. Flush changes:  
`systemctl daemon-reload`  
4. Verify that the configuration has been loaded:  
`systemctl show --property=Environment docker`  
You should see:  
`Environment=HTTP_PROXY=http://localhost:3128/ HTTPS_PROXY=http://localhost:3128/`
5. Restart Docker:  
`systemctl restart docker`

#### Adding Proxy variables to the runner config, so it get's builds assigned from the gitlab behind the proxy  
This is basically the same as adding the proxy to docker.  

1. Create a systemd drop-in directory for the gitlab-runner service:  
`mkdir /etc/systemd/system/gitlab-runner.service.d`  
2. Create a file called `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf` that adds the `HTTP_PROXY` environment variable:  

```
[Service]
Environment="HTTP_PROXY=http://localhost:3128/"
Environment="HTTPS_PROXY=http://localhost:3128/"
```  

3. Flush changes:  
`systemctl daemon-reload`  
4. Verify that the configuration has been loaded:  
`systemctl show --property=Environment gitlab-runner`  
You should see:  
`Environment=HTTP_PROXY=http://localhost:3128/ HTTPS_PROXY=http://localhost:3128/`


#### adding the proxy to the docker containers (for git clone and other stuff)  
After you registered your runner, you might want to propagate your proxy settings to the docker containers.   
To do that you need to edit the `/etc/gitlab-runner/config.toml` and add the following to the `[[runners]]` section:  
```
pre_clone_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["HTTPS_PROXY=<your_host_ip_here>:3128", "HTTP_PROXY=<your_host_ip_here>:3128"]