package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

// dockerClient is the real dockerAPI, backed by the official Docker Engine SDK
// (github.com/moby/moby/client).
type dockerClient struct {
	cli *client.Client
	ctx context.Context
}

// newDockerClient connects to the daemon (honoring DOCKER_HOST, DOCKER_CONTEXT,
// and TLS env) and pings it so connection failures surface immediately.
func newDockerClient() (*dockerClient, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		cli.Close()
		if client.IsErrConnectionFailed(err) {
			return nil, fmt.Errorf("cannot connect to Docker daemon: %w", err)
		}
		return nil, err
	}
	return &dockerClient{cli: cli, ctx: ctx}, nil
}

func (d *dockerClient) Close() error { return d.cli.Close() }

// List returns the resources of one kind, sorted by name.
func (d *dockerClient) List(k kind) ([]item, error) {
	switch k {
	case kindContainers:
		return d.listContainers()
	case kindImages:
		return d.listImages()
	case kindVolumes:
		return d.listVolumes()
	}
	return nil, fmt.Errorf("unknown resource kind %d", k)
}

func (d *dockerClient) listContainers() ([]item, error) {
	res, err := d.cli.ContainerList(d.ctx, client.ContainerListOptions{All: true, Size: true})
	if err != nil {
		return nil, err
	}
	out := make([]item, 0, len(res.Items))
	for _, c := range res.Items {
		out = append(out, item{
			id:      c.ID,
			name:    firstName(c.Names),
			image:   c.Image,
			status:  c.Status,
			created: c.Created,
			size:    c.SizeRw,
			running: c.State == "running",
		})
	}
	sortItems(out)
	return out, nil
}

func (d *dockerClient) listImages() ([]item, error) {
	res, err := d.cli.ImageList(d.ctx, client.ImageListOptions{All: true})
	if err != nil {
		return nil, err
	}
	byImage, err := d.containersByImage()
	if err != nil {
		return nil, err
	}

	out := make([]item, 0, len(res.Items))
	for _, im := range res.Items {
		name := "<none>"
		if len(im.RepoTags) > 0 {
			name = im.RepoTags[0]
		}
		used := byImage[im.ID]
		out = append(out, item{
			id:      im.ID,
			name:    name,
			image:   strings.Join(im.RepoTags, ", "),
			status:  humanSize(im.Size),
			created: im.Created,
			size:    im.Size,
			inUse:   len(used) > 0,
			usedBy:  used,
		})
	}
	sortItems(out)
	return out, nil
}

func (d *dockerClient) listVolumes() ([]item, error) {
	res, err := d.cli.VolumeList(d.ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, err
	}
	byVol, err := d.containersByVolume()
	if err != nil {
		return nil, err
	}

	out := make([]item, 0, len(res.Items))
	for _, v := range res.Items {
		var size int64
		if v.UsageData != nil && v.UsageData.Size > 0 {
			size = v.UsageData.Size
		}
		used := byVol[v.Name]
		out = append(out, item{
			id:      v.Name,
			name:    v.Name,
			image:   v.Driver,
			status:  fmt.Sprintf("%d refs", len(used)),
			created: parseRFC3339(v.CreatedAt),
			size:    size,
			inUse:   len(used) > 0,
			usedBy:  used,
		})
	}
	sortItems(out)
	return out, nil
}

// Remove deletes one resource. force is used for running containers and
// in-use images/volumes; the confirm screen guarantees the user asked for it.
func (d *dockerClient) Remove(k kind, id string, force bool) error {
	switch k {
	case kindContainers:
		_, err := d.cli.ContainerRemove(d.ctx, id, client.ContainerRemoveOptions{Force: force})
		return err
	case kindImages:
		_, err := d.cli.ImageRemove(d.ctx, id, client.ImageRemoveOptions{Force: force, PruneChildren: true})
		return err
	case kindVolumes:
		_, err := d.cli.VolumeRemove(d.ctx, id, client.VolumeRemoveOptions{Force: force})
		return err
	}
	return fmt.Errorf("unknown resource kind %d", k)
}

// containersByImage maps image config-digest -> names of containers using it.
// The daemon does not compute image.Summary.Containers, so this join is the
// reliable "in use" signal for images.
func (d *dockerClient) containersByImage() (map[string][]string, error) {
	res, err := d.cli.ContainerList(d.ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	byImage := map[string][]string{}
	for _, c := range res.Items {
		byImage[c.ImageID] = append(byImage[c.ImageID], firstName(c.Names))
	}
	return byImage, nil
}

// containersByVolume maps volume name -> names of containers mounting it.
func (d *dockerClient) containersByVolume() (map[string][]string, error) {
	res, err := d.cli.ContainerList(d.ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	byVol := map[string][]string{}
	for _, c := range res.Items {
		for _, m := range c.Mounts {
			if m.Name != "" {
				byVol[m.Name] = append(byVol[m.Name], firstName(c.Names))
			}
		}
	}
	return byVol, nil
}

// firstName strips the leading "/" Docker prepends to container names.
func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func sortItems(items []item) {
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
}

func parseRFC3339(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.Unix()
}
