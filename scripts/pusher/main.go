package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/errgroup"
)

type Manifest struct {
	Dir    string
	Export string

	Default map[string][]string
	Match   map[string]map[string][]string
}

type Export struct {
	Type  string
	Value string
}

var dry bool

func main() {
	flag.BoolVar(&dry, "dry-run", false, "dry-run")
	flag.Parse()

	if flag.NArg() < 3 {
		fmt.Println("usage: <manifest> <repo> [tag...]")
		os.Exit(1)
	}

	manifest := flag.Arg(0)
	repo := flag.Arg(1)
	tags := flag.Args()[2:]

	fmt.Println(manifest, repo, tags)

	var m Manifest
	buf, err := os.ReadFile(manifest)
	if err != nil {
		fmt.Printf("error reading manifest: %v", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		fmt.Printf("error unmarshaling: %v", err)
		os.Exit(1)
	}

	var exports []Export
	images := map[string][]string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		for archive, names := range match(m, tag) {
			// rewrite names
			var taggedNames []string
			for _, name := range names {
				taggedName := strings.ReplaceAll(name, "%", tag)
				taggedNames = append(taggedNames, taggedName)
				exports = append(exports, Export{Type: "Docker image", Value: repo + ":" + taggedName})
			}

			archive = filepath.Join(m.Dir, archive+".tar")
			images[archive] = append(images[archive], taggedNames...)
		}
	}

	// export before we do the work
	pathname := filepath.Join(m.Export, strings.NewReplacer("/", "_", "\\", "_", ".", "_").Replace(manifest+"-"+repo+"-"+strings.Join(tags, "-"))+".json")
	{
		exported, err := json.Marshal(exports)
		if err != nil {
			panic(err)
		}
		os.MkdirAll(m.Export, 0o777)
		if err := os.WriteFile(pathname, exported, 0o600); err != nil {
			fmt.Printf("error writing export: %v", err)
			os.Exit(1)
		}
	}

	now := time.Now()

	wg, ctx := errgroup.WithContext(context.Background())
	wg.SetLimit(8)

	for archive, names := range images {
		wg.Go(func() error {
			return push(ctx, archive, repo, names)
		})
	}

	if err := wg.Wait(); err != nil {
		fmt.Printf("error pushing: %v", err)
		os.Exit(1)
	}

	fmt.Printf("done in %v, export %v\n", time.Since(now), pathname)
}

func match(m Manifest, tag string) map[string][]string {
	if match, ok := m.Match[tag]; ok {
		return match
	}

	return m.Default
}

func push(ctx context.Context, src, repo string, tags []string) error {
	pusher, err := remote.NewPusher(remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return fmt.Errorf("creating pusher: %w", err)
	}

	dir, err := extract(src)
	if err != nil {
		return fmt.Errorf("extracting oci-layout tar: %w", err)
	}
	defer os.RemoveAll(dir)

	// fix oci archive
	if err := fixOCIArchive(dir); err != nil {
		return fmt.Errorf("fixing archive %v: %w", dir, err)
	}

	ociLayout, err := layout.FromPath(dir)
	if err != nil {
		return fmt.Errorf("opening oci-layout: %w", err)
	}

	index, err := ociLayout.ImageIndex()
	if err != nil {
		return fmt.Errorf("getting image index: %w", err)
	}

	for _, tag := range tags {
		ref, err := name.ParseReference(repo + ":" + tag)
		if err != nil {
			return fmt.Errorf("parsing dst reference: %w", err)
		}

		fmt.Printf("[%v] %v => %v\n", src, repo, tag)

		if dry {
			continue
		}

		now := time.Now()
		if err := pusher.Push(ctx, ref, index); err != nil {
			return fmt.Errorf("pusing image %v: %w", ref, err)
		}

		fmt.Printf("[%v] %v => %v (%v)\n", src, repo, tag, time.Since(now))
	}

	return nil
}

func extract(archive string) (dir string, err error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "tar-extract-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tempDir)
		}
	}()

	tarReader := tar.NewReader(f)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// ignore non-files, they're not found in oci-layout
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		if err := func() error {
			targetPath := filepath.Join(tempDir, hdr.Name)

			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			file, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(file, tarReader); err != nil {
				return err
			}

			return file.Close()
		}(); err != nil {
			return "", err
		}
	}

	return tempDir, nil
}

// fixOCIArchive fixes an oci layout directory for multi-arch images built by
// buildx.
//
// In some scenarios, Buildx incorrectly uses an image index manifest for
// index.json. Whilst this works for many tools, including Docker, it breaks
// Podman and Docker Hub struggles with it  (failing to display each arch in
// the image).
//
// This can be easily fixed by copying the references blob to index.json if
// it is the image manifest we expect.
func fixOCIArchive(dir string) error {
	indexPath := filepath.Join(dir, "index.json")

	index, err := os.Open(indexPath)
	if err != nil {
		return fmt.Errorf("opening index: %w", err)
	}
	defer index.Close()

	indexManifest, err := v1.ParseIndexManifest(index)
	if err != nil {
		return err
	}

	// only proceed if we get one manifest
	if len(indexManifest.Manifests) > 1 {
		return nil
	}
	if !indexManifest.Manifests[0].MediaType.IsIndex() {
		return nil
	}

	digest := indexManifest.Manifests[0].Digest
	blobPath := filepath.Join(dir, "blobs", digest.Algorithm, digest.Hex)
	imageIndex, err := os.Open(blobPath)
	if err != nil {
		return err
	}

	indexManifest, err = v1.ParseIndexManifest(imageIndex)
	if err != nil {
		return err
	}

	// only proceed if we get an image manifest
	if len(indexManifest.Manifests) == 0 || !indexManifest.Manifests[0].MediaType.IsImage() {
		return nil
	}

	if err := os.Remove(indexPath); err != nil {
		return err
	}
	if err := os.Rename(blobPath, indexPath); err != nil {
		return err
	}

	return nil
}
