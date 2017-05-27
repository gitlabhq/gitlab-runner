package digital_ocean

import (
	"time"
	"io/ioutil"
	"context"
	"errors"

	"github.com/digitalocean/godo"
	"github.com/cenkalti/backoff"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

type dropletInstance struct {
	droplet *godo.Droplet
	client *godo.Client
}

func (d *dropletInstance) waitForRunning(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()

	for {
		droplet, resp, err := d.client.Droplets.Get(ctx, d.droplet.ID)
		if isNotFound(resp) {
			return err
		} else if retry(resp, backoff) {
			continue
		} else if err != nil {
			return err
		}

		if droplet.Status == "active" {
			d.droplet = droplet
			return nil
		}

		time.Sleep(10 * time.Second)
	}
	return nil
}

func (d *dropletInstance) Create(ctx context.Context, name string, tags []string, config *common.DigitalOceanConfig) (error) {
	createRequest := &godo.DropletCreateRequest{
		Image:             godo.DropletCreateImage{Slug: config.Image},
		Name:              name,
		Region:            config.Region,
		Size:              config.Size,
		IPv6:              config.IPv6,
		PrivateNetworking: config.PrivateNetworking,
		Backups:           config.Backups,
		SSHKeys:           []godo.DropletCreateSSHKey{{Fingerprint: config.SSHFingerprint}},
		Tags:              append(config.Tags, tags...),
	}

	if config.UserData != "" {
		buf, err := ioutil.ReadFile(config.UserData)
		if err != nil {
			return err
		}
		createRequest.UserData = string(buf)
	}

	droplet, _, err := d.client.Droplets.Create(ctx, createRequest)
	if err != nil {
		return err
	}

	d.droplet = droplet
	return nil
}

func (d *dropletInstance) GetName() string {
	return d.droplet.Name
}

func (d *dropletInstance) GetIP() string {
	for _, network := range d.droplet.Networks.V4 {
		if network.Type == "public" {
			return network.IPAddress
		}
	}
	return ""
}

func (d *dropletInstance) Remove(timeout time.Duration) (lastErr error) {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	backoff := backoff.NewExponentialBackOff()

	lastErr = errors.New("failed to remove")

	for lastErr != nil {
		resp, err := d.client.Droplets.Delete(ctx, d.droplet.ID)
		if err == context.DeadlineExceeded {
			break
		} else if isNotFound(resp) {
			return nil
		} else if retry(resp, backoff) {
			continue
		} else {
			lastErr = err
		}
	}
	return
}

func (d *dropletInstance) Reinstall(timeout time.Duration) (lastErr error) {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
	defer cancelFn()

	backoff := backoff.NewExponentialBackOff()

	lastErr = errors.New("failed to reinstall")

	for {
		_, resp, err := d.client.DropletActions.RebuildByImageSlug(ctx, d.droplet.ID, d.droplet.Image.Slug)
		if err == context.DeadlineExceeded {
			break
		} else if isNotFound(resp) {
			return nil
		} else if retry(resp, backoff) {
			continue
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return
	}

	return d.waitForRunning(ctx)
}

func (d *dropletInstance) Valid() bool {
	return d.droplet.Status == "active"
}
