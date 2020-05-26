package docker

import "github.com/docker/docker/api/types"

func newDockerTTY(hijackedResp *types.HijackedResponse) *dockerTTY {
	return &dockerTTY{
		hijackedResp: hijackedResp,
	}
}

type dockerTTY struct {
	hijackedResp *types.HijackedResponse
}

func (d *dockerTTY) Read(p []byte) (int, error) {
	return d.hijackedResp.Reader.Read(p)
}

func (d *dockerTTY) Write(p []byte) (int, error) {
	return d.hijackedResp.Conn.Write(p)
}

func (d *dockerTTY) Close() error {
	d.hijackedResp.Close()
	_ = d.hijackedResp.CloseWrite()
	return nil
}
