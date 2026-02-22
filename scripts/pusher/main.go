package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
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
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/errgroup"
)

type Manifest struct {
	Dir    string
	Export string

	Indices []PusherIndexManifest
	Default map[string][]string
	Match   map[string]map[string][]string
}

type Export struct {
	Type  string
	Value string
}

type PusherIndexManifest struct {
	Tags       []string
	Components []string
}

type ArchiveDescriptor struct {
	archive string
	ref     name.Reference
}

type ArchiveDescriptorMap map[string]ArchiveDescriptor

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
		fmt.Printf("error reading manifest: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		fmt.Printf("error unmarshaling: %v\n", err)
		os.Exit(1)
	}

	if err := validate(m, tags); err != nil {
		fmt.Printf("error validating manifest:\n%v\n", err)
		os.Exit(1)
	}

	var exports []Export
	images := map[string][]string{}
	indexTags := make([][]string, len(m.Indices))
	nameToArchive := map[string]string{}
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

			archivePath := filepath.Join(m.Dir, archive+".tar")
			// capture the mapping from short name to archive path, for use
			// when pushing images
			nameToArchive[archive] = archivePath
			images[archivePath] = append(images[archivePath], taggedNames...)
		}

		for i, indexDef := range m.Indices {
			var taggedNames []string
			for _, indexTag := range indexDef.Tags {
				taggedName := strings.ReplaceAll(indexTag, "%", tag)
				taggedNames = append(taggedNames, taggedName)
				// Ideally, this would be flagged as "Docker image index" or similar, but
				// there's work left to do to flag all indexes as such in order to get
				// artifact validation working properly.
				exports = append(exports, Export{Type: "Docker image", Value: repo + ":" + taggedName})
			}
			indexTags[i] = append(indexTags[i], taggedNames...)
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
	archiveDescriptors := pushImages(repo, images)
	pushIndices(repo, m, nameToArchive, archiveDescriptors, indexTags)
	fmt.Printf("done in %v, export %v\n", time.Since(now), pathname)
}

func pushImages(repo string, images map[string][]string) ArchiveDescriptorMap {
	now := time.Now()

	imageInfoCh := make(chan ArchiveDescriptor, len(images))
	wg, ctx := errgroup.WithContext(context.Background())
	wg.SetLimit(8)

	for archive, names := range images {
		wg.Go(func() error {
			return push(ctx, archive, repo, imageInfoCh, names)
		})
	}

	if err := wg.Wait(); err != nil {
		fmt.Printf("error pushing: %v", err)
		os.Exit(1)
	}
	close(imageInfoCh)
	archiveDescriptors := ArchiveDescriptorMap{}
	for descriptor := range imageInfoCh {
		archiveDescriptors[descriptor.archive] = descriptor
	}
	fmt.Printf("done pushing images in %v\n", time.Since(now))
	return archiveDescriptors
}

func pushIndices(repo string, m Manifest, nameToArchive map[string]string, archiveDescriptors ArchiveDescriptorMap, indexTags [][]string) {
	if len(m.Indices) == 0 {
		fmt.Printf("No index pushes configured")
		return
	}

	now := time.Now()

	wg, ctx := errgroup.WithContext(context.Background())
	wg.SetLimit(8)

	for i, names := range indexTags {
		wg.Go(func() error {
			return pushIndex(ctx, nameToArchive, m.Indices[i], archiveDescriptors, repo, names)
		})
	}

	if err := wg.Wait(); err != nil {
		fmt.Printf("error pushing: %v", err)
		os.Exit(1)
	}

	fmt.Printf("done pushing indices in %v\n", time.Since(now))
}

func match(m Manifest, tag string) map[string][]string {
	if match, ok := m.Match[tag]; ok {
		return match
	}

	return m.Default
}

func push(ctx context.Context, src, repo string, imageCh chan ArchiveDescriptor, tags []string) error {
	if len(tags) == 0 {
		return fmt.Errorf("refusing to push with no tags")
	}
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

	if err != nil {
		return fmt.Errorf("getting index manifest: %w", err)
	}

	// Leaking the ref outside of this loop because we want a _ref_ to associate
	// with the archive. We don't care which one, as it's just a pointer to data,
	// and all the pushed refs point to the same data.
	// It might be nice to grab a more appropriate ref (e.g. the sha-tagged ref
	// rather than `bleeding` to protect against concurrent builds.
	// Maybe rework the communication so that we use the digest rather than the tag?
	var ref name.Reference
	for _, tag := range tags {
		ref, err = name.ParseReference(repo + ":" + tag)
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

	imageCh <- ArchiveDescriptor{
		archive: src,
		ref:     ref,
	}
	return nil
}

func pushIndex(
	ctx context.Context,
	nameToArchive map[string]string,
	src PusherIndexManifest,
	archiveDescriptors ArchiveDescriptorMap,
	repo string,
	tags []string,
) error {
	if len(src.Components) == 0 {
		fmt.Printf("Doing nothing for index: %s, %v\n", repo, tags)
		return nil
	}

	authOpt := remote.WithAuthFromKeychain(authn.DefaultKeychain)
	pusher, err := remote.NewPusher(authOpt)
	if err != nil {
		return fmt.Errorf("creating pusher: %w", err)
	}

	// The pattern is to start with an empty Index (which we're trusting to
	// have a sensible default schema version), and apply mutations to
	// get to the desired state.
	var index v1.ImageIndex
	index = empty.Index
	mutations := make([]mutate.IndexAddendum, 0, len(src.Components))
	for _, component := range src.Components {
		archivePath := nameToArchive[component]
		archiveDesc := archiveDescriptors[archivePath]

		desc, err := remote.Get(archiveDesc.ref, authOpt)
		if err != nil {
			return fmt.Errorf("calling get: %w", err)
		}

		var image v1.Image
		if desc.MediaType.IsIndex() {
			idx, err := desc.ImageIndex()
			if err != nil {
				return fmt.Errorf("translating remote descriptor to an image index: %w", err)
			}

			indexMan, err := idx.IndexManifest()
			if err != nil {
				return fmt.Errorf("pulling manifest from the fetched source image: %w", err)
			}

			if len(indexMan.Manifests) != 1 || !indexMan.Manifests[0].MediaType.IsImage() {
				return fmt.Errorf("expected single image in the manifest")
			}

			imageMan := indexMan.Manifests[0]
			image, err = idx.Image(imageMan.Digest)
			if err != nil {
				return fmt.Errorf("dereferencing the image from the manifest: %w", err)
			}
			mutations = append(mutations, mutate.IndexAddendum{
				Add:        image,
				Descriptor: imageMan,
			})
		} else {
			return fmt.Errorf("didn't expect non-index: %s", archiveDesc.ref)
		}
	}
	index = mutate.AppendManifests(index, mutations...)

	if dry {
		return nil
	}

	for _, tag := range tags {
		idxRef, err := name.ParseReference(repo + ":" + tag)
		if err != nil {
			return err
		}
		fmt.Printf("%q %s => %s\n", src.Components, repo, tag)
		pusher.Push(ctx, idxRef, index)

	}
	return nil
}

func extract(archive string) (dir string, err error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()

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
	defer imageIndex.Close()

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

func validate(m Manifest, tags []string) error {
	// Cross reference archives across indices and the raw images
	// being pushed
	var errs []error
	// This tags is the tag fragments given on the command line, e.g. latest, bleeding, sha
	for _, tag := range tags {
		pushedImages := match(m, tag)
		for i, pusherIndex := range m.Indices {
			// Not currently validating alignment of tags, e.g. it's
			// valid to push an repo:index pointing to [repo:arm-foo, repo:amd64-bar]
			// Also not validating that we don't have platform collisions, as that
			// would be rather expensive to check at this point
			for _, comp := range pusherIndex.Components {
				if imageTags, ok := pushedImages[comp]; !ok || len(imageTags) == 0 {
					errs = append(errs, fmt.Errorf("index %d references component %s not pushed under tag %s", i, comp, tag))
				}
			}
		}
	}
	return errors.Join(errs...)
}
