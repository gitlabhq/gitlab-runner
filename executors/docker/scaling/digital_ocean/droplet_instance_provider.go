package digital_ocean

import (
	"errors"
	"context"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
	"github.com/cenkalti/backoff"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/docker/scaling"

	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/docker"
)

type dropletInstanceProvider struct {
}

func (d *dropletInstanceProvider) getClient(accessToken string) *godo.Client {
	token := &oauth2.Token{AccessToken: accessToken}
	tokenSource := oauth2.StaticTokenSource(token)
	client := oauth2.NewClient(oauth2.NoContext, tokenSource)
	return godo.NewClient(client)
}

func (d *dropletInstanceProvider) Create(name string, config *common.RunnerConfig) (scaling.Instance, error) {
	if config.DigitalOcean == nil {
		return nil, errors.New("missing digital ocean configuration")
	}

	timeout := config.DigitalOcean.CreationTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	i := &dropletInstance{
		client: d.getClient(config.DigitalOcean.Token),
	}

	tags := []string{}
	tags = append(tags, "runner-" + config.ShortDescription())

	err := i.Create(ctx, name, tags, config.DigitalOcean)
	if err != nil {
		return nil, err
	}

	err = i.waitForRunning(ctx)
	if err != nil {
		go i.Remove(config.DigitalOcean.RemovalTimeout)
	}
	return i, nil
}

func (d *dropletInstanceProvider) List(config *common.RunnerConfig) ([]scaling.Instance, error) {
	if config.DigitalOcean == nil {
		return nil, errors.New("missing digital ocean configuration")
	}

	client := d.getClient(config.DigitalOcean.Token)

	options := &godo.ListOptions{
		PerPage: 10000,
	}

	var droplets []godo.Droplet

	backoff := backoff.NewExponentialBackOff()

	for {
		var resp *godo.Response
		var err error
		droplets, resp, err = client.Droplets.List(context.TODO(), options)
		if retry(resp, backoff) {
			continue
		} else if err != nil {
			return nil, err
		}
	}

	var instances []scaling.Instance

	runnerTag := "runner-" + config.ShortDescription()

	for _, droplet := range droplets {
		for _, tag := range droplet.Tags {
			if tag == runnerTag {
				instances = append(instances, &dropletInstance{
					client: client,
					droplet: &droplet,
				})
				break
			}
		}
	}

	return instances, nil
}

func init() {
	scaling.NewMachineProvider("docker+digitalocean", "docker", &dropletInstanceProvider{})
}
