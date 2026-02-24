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

// Represents our configuration file. Includes json metadata to
// facilitate formatting when printing the configuration.
type Manifest struct {
	Dir     string                         `json:"dir"`
	Export  string                         `json:"export"`
	Indexes []ImageIndex                   `json:"indexes,omitempty"`
	Default map[string][]string            `json:"default"`
	Match   map[string]map[string][]string `json:"match,omitempty"`
}

type Export struct {
	Type  string
	Value string
}

// Hold the processed configuration, combining the config file with command line arguments.
type RuntimeConfig struct {
	manifestPath    string
	manifest        Manifest
	repo            string
	tagFragments    []string
	exports         []Export
	imageTags       map[string][]string
	indexTags       []ImageIndex
	componentRefMap map[string]name.Reference
	authOpt         remote.Option
}

// Captures the component name and a digest-based reference to the resulting image contents
// Used to pass these details from the initial image push to the eventual image index push.
type ComponentRef struct {
	name string
	ref  name.Reference
}

var dry, generateIndexes, printIndexes bool

func main() {
	flag.BoolVar(&dry, "dry-run", false, "print what would be done, but don't push anything")
	flag.BoolVar(&generateIndexes, "gen-indexes", false, "generate index configuration before pushing")
	flag.BoolVar(&printIndexes, "print-indexes", false, "print generated index config and exit")
	flag.Parse()

	if flag.NArg() < 3 && (!printIndexes || flag.NArg() < 1) {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		fmt.Printf("%s [-gen-indexes] <manifest> <repo> [tag...]  Push configured images and indexes to repo with specified tags\n", filepath.Base(os.Args[0]))
		fmt.Printf("%s -print-indexes <manifest>                  Print auto-generated index config to stdout, then exit\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	// Split arg processing. If we're only printing the config, just read the
	// manifest, print the result and exit.
	manifest := flag.Arg(0)
	m, err := readManifest(manifest)
	if err != nil {
		fmt.Printf("error %v\n", err)
		os.Exit(1)
	}

	if printIndexes {
		indexes := GenerateIndexes(&m)
		output, err := json.MarshalIndent(indexes, "", "    ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error marshaling output: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
		os.Exit(0)
	}

	// Not just printing config, so continue normal arg processing
	repo := flag.Arg(1)
	tags := flag.Args()[2:]

	fmt.Println(manifest, repo, tags)

	config := newRuntimeConfig(manifest, m, repo, tags)

	// Validate that index components are included in the images being pushed.
	if err := config.validate(); err != nil {
		fmt.Printf("error validating manifest:\n%v\n", err)
		os.Exit(1)
	}

	// export before we do the work
	pathname, err := config.writeExports()
	if err != nil {
		fmt.Printf("error writing exports: %v", err)
		os.Exit(1)
	}

	now := time.Now()
	if err = config.pushImages(); err != nil {
		fmt.Printf("error pushing component images: %v", err)
		os.Exit(1)
	}
	if err = config.pushIndexes(); err != nil {
		fmt.Printf("error pushing indexes: %v", err)
		os.Exit(1)
	}
	fmt.Printf("done in %v, export %v\n", time.Since(now), pathname)
}

func readManifest(manifestPath string) (Manifest, error) {
	var m Manifest
	buf, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading manifest: %w", err)
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		return Manifest{}, fmt.Errorf("unmarshaling: %w", err)
	}

	// If requested, generate indexes from the Default map rather than using
	// those in the config file.
	if generateIndexes {
		m.Indexes = GenerateIndexes(&m)
	}
	return m, nil
}

// Push all the configured component images.
//
// While pushing images, the runtime config is updated to track the
// digests of pushed images, so they can be referenced when pushing
// the indexes.
func (c *RuntimeConfig) pushImages() error {
	now := time.Now()

	imageInfoCh := make(chan ComponentRef, len(c.imageTags))
	wg, ctx := errgroup.WithContext(context.Background())
	wg.SetLimit(8)

	for component := range c.imageTags {
		wg.Go(func() error {
			return c.pushImage(ctx, component, imageInfoCh)
		})
	}

	if err := wg.Wait(); err != nil {
		return fmt.Errorf("pushing images: %w", err)
	}
	close(imageInfoCh)
	for componentRef := range imageInfoCh {
		c.registerComponentRef(componentRef)
	}
	fmt.Printf("done pushing images in %v\n", time.Since(now))
	return nil
}

func (c *RuntimeConfig) registerComponentRef(compRef ComponentRef) {
	c.componentRefMap[compRef.name] = compRef.ref
}

func (c RuntimeConfig) refForComponent(name string) (name.Reference, error) {
	if ref, ok := c.componentRefMap[name]; ok {
		return ref, nil
	}
	return nil, fmt.Errorf("no ref for component: %s", name)
}

// Push a component image to the specified repo with configured tags. Additionally capture the
// digest ref of the pushed image in an ArchiveDescriptor, and pass it via the image channel to
// the caller, so that reference can be used when later building the index.
func (c *RuntimeConfig) pushImage(ctx context.Context, componentName string, imageCh chan ComponentRef) error {
	tags := c.imageTags[componentName]
	if len(tags) == 0 {
		return fmt.Errorf("refusing to push with no tags")
	}
	pusher, err := remote.NewPusher(c.authOpt)
	if err != nil {
		return fmt.Errorf("creating pusher: %w", err)
	}

	archive := c.manifest.archivePath(componentName)
	dir, err := extract(archive)
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

	// We create a digestRef as a handle for the later index push. We prefer the
	// digest-based ref instead of a tag-based ref to protect against concurrent
	// builds each modifying "bleeding" or "latest" style tags.
	digest, err := index.Digest()
	if err != nil {
		return fmt.Errorf("getting digest for index: %w", err)
	}
	digestRef, err := name.ParseReference(c.repo + "@" + digest.String())
	if err != nil {
		return fmt.Errorf("parsing digest ref to dest: %w", err)
	}
	for _, tag := range tags {
		ref, err := name.ParseReference(c.repo + ":" + tag)
		if err != nil {
			return fmt.Errorf("parsing tag ref to dest: %w", err)
		}
		fmt.Printf("[%v] %v => %v\n", archive, c.repo, tag)

		if dry {
			continue
		}

		now := time.Now()
		if err := pusher.Push(ctx, ref, index); err != nil {
			return fmt.Errorf("pushing image %v: %w", ref, err)
		}
		fmt.Printf("[%v] %v => %v@%s (%v)\n", archive, c.repo, tag, digest, time.Since(now))
	}

	// Now that we've pushed the image, send the digestRef back to the caller
	imageCh <- ComponentRef{
		name: componentName,
		ref:  digestRef,
	}
	return nil
}

// Push all indexes as configured
func (c *RuntimeConfig) pushIndexes() error {
	if len(c.indexTags) == 0 {
		fmt.Println("No index pushes configured")
		return nil
	}

	now := time.Now()

	wg, ctx := errgroup.WithContext(context.Background())
	wg.SetLimit(8)

	for _, imageIndex := range c.indexTags {
		wg.Go(func() error {
			return c.pushIndex(ctx, imageIndex)
		})
	}

	if err := wg.Wait(); err != nil {
		return fmt.Errorf("pushing indexes: %w", err)
	}

	fmt.Printf("done pushing indexes in %v\n", time.Since(now))
	return nil
}

// Push a single image index to the repo.
//
// Given our representation of an ImageIndex, build the v1.ImageIndex from our captured
// archive details, and push that v1.ImageIndex to the repo and tags provided.
func (c *RuntimeConfig) pushIndex(
	ctx context.Context,
	src ImageIndex,
) error {
	if len(src.Components) == 0 {
		fmt.Printf("Doing nothing for index: %s, %v\n", c.repo, src.Tags)
		return nil
	}

	pusher, err := remote.NewPusher(c.authOpt)
	if err != nil {
		return fmt.Errorf("creating pusher: %w", err)
	}

	index, err := c.buildIndexForPush(src)
	if err != nil {
		return fmt.Errorf("building index for push: %w", err)
	}

	for _, tag := range src.Tags {
		idxRef, err := name.ParseReference(c.repo + ":" + tag)
		if err != nil {
			return fmt.Errorf("parsing index ref for push: %w", err)
		}
		fmt.Printf("%q %s => %s\n", src.Components, c.repo, tag)

		if dry {
			continue
		}

		if err := pusher.Push(ctx, idxRef, index); err != nil {
			return fmt.Errorf("pushing image index %v: %w", idxRef, err)
		}
	}
	return nil
}

func (c RuntimeConfig) buildIndexForPush(src ImageIndex) (v1.ImageIndex, error) {
	// The pattern is to start with an empty Index (which we're trusting to
	// have a sensible default schema version), and apply mutations to
	// get to the desired state.
	var index v1.ImageIndex = empty.Index
	mutations := make([]mutate.IndexAddendum, 0, len(src.Components))
	for _, component := range src.Components {
		if dry {
			continue
		}
		ref, err := c.refForComponent(component)
		if err != nil {
			return index, err
		}

		indexAddendum, err := c.buildIndexComponentAddendum(ref)
		if err != nil {
			return index, fmt.Errorf("building index entry for %s: %w", component, err)
		}

		mutations = append(mutations, *indexAddendum)
	}
	return mutate.AppendManifests(index, mutations...), nil
}

// Given a name reference, fetch the manifest for the component from the remote repo,
// and package it into an IndexAddendum suitable to be appended to the index being pushed.
//
// We fetch details from the remote because the extracted local archive has been
// removed by the time we push the index.
func (c RuntimeConfig) buildIndexComponentAddendum(ref name.Reference) (*mutate.IndexAddendum, error) {
	desc, err := remote.Get(ref, c.authOpt)
	if err != nil {
		return nil, fmt.Errorf("calling get: %w", err)
	}

	if !desc.MediaType.IsIndex() {
		return nil, fmt.Errorf("didn't expect non-index: %s", ref)
	}

	idx, err := desc.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("translating remote descriptor to an image index: %w", err)
	}

	indexMan, err := idx.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("pulling manifest from the fetched source image: %w", err)
	}

	if len(indexMan.Manifests) != 1 || !indexMan.Manifests[0].MediaType.IsImage() {
		return nil, fmt.Errorf("expected single image in the manifest")
	}

	imageMan := indexMan.Manifests[0]
	image, err := idx.Image(imageMan.Digest)
	if err != nil {
		return nil, fmt.Errorf("dereferencing the image from the manifest: %w", err)
	}

	return &mutate.IndexAddendum{
		Add:        image,
		Descriptor: imageMan,
	}, nil
}

// Extract the given archive.
//
// The layout methods require an extracted archive, so we extract to a temp dir and return
// the path to that dir. The caller is responsible for cleaning up the temp dir when it's
// no longer needed.
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

// Process the command line arguments and manifest file.
//
// Primarily, this means combining the tag fragments provided on the command line with
// the tag templates given in the configuration file, to create the final set of concrete
// tags to push for each component and index.
func newRuntimeConfig(manifestPath string, manifest Manifest, repo string, tagFragments []string) RuntimeConfig {
	var exports []Export
	tagFragments = cleanTagFragments(tagFragments)

	imageTags, imageExports := manifest.buildImageTags(repo, tagFragments)
	indexTags, indexExports := manifest.buildIndexTags(repo, tagFragments)

	exports = append(exports, imageExports...)
	exports = append(exports, indexExports...)

	return RuntimeConfig{
		manifestPath:    manifestPath,
		manifest:        manifest,
		repo:            repo,
		tagFragments:    tagFragments,
		exports:         exports,
		imageTags:       imageTags,
		indexTags:       indexTags,
		componentRefMap: map[string]name.Reference{},
		authOpt:         remote.WithAuthFromKeychain(authn.DefaultKeychain),
	}
}

func cleanTagFragments(tagFragments []string) []string {
	var cleanedFragments []string
	for _, tagFragment := range tagFragments {
		tagFragment = strings.TrimSpace(tagFragment)
		if tagFragment == "" {
			continue
		}
		cleanedFragments = append(cleanedFragments, tagFragment)
	}
	return cleanedFragments
}

func (m Manifest) buildImageTags(repo string, tagFragments []string) (map[string][]string, []Export) {
	var exports []Export
	imageTags := map[string][]string{}
	for _, tagFragment := range tagFragments {
		for component, tagTemplates := range m.match(tagFragment) {
			for _, tagTemplate := range tagTemplates {
				tag := strings.ReplaceAll(tagTemplate, "%", tagFragment)
				imageTags[component] = append(imageTags[component], tag)
				exports = append(exports, Export{Type: "Docker image", Value: repo + ":" + tag})
			}
		}
	}
	return imageTags, exports
}

func (m Manifest) buildIndexTags(repo string, tagFragments []string) ([]ImageIndex, []Export) {
	// For index config processing, the input ImageIndex from manifest.Indexes
	// pairs tag templates (e.g. ubuntu-%) with the components to be included.
	// The output ImageIndex values pair populated tags with those components,
	// (e.g. [ubuntu-bleeding, ubuntu-latest]).
	var indexTags []ImageIndex
	var exports []Export

	for _, indexDef := range m.Indexes {
		var tags []string
		for _, tagFragment := range tagFragments {
			for _, indexTagTemplate := range indexDef.Tags {
				tag := strings.ReplaceAll(indexTagTemplate, "%", tagFragment)
				tags = append(tags, tag)
				// Ideally, this would be flagged as "Docker image index" or similar,
				// but we have some difficulty in differentiating between images and
				// image indexes, since all artifacts pushed are currently image indexes.
				// The component images are pushed as indexes with a single manifest.
				exports = append(exports, Export{Type: "Docker image", Value: repo + ":" + tag})
			}
		}
		indexTags = append(indexTags, ImageIndex{Tags: tags, Components: indexDef.Components})
	}
	return indexTags, exports
}

func (m Manifest) match(tag string) map[string][]string {
	if match, ok := m.Match[tag]; ok {
		return match
	}

	return m.Default
}

func (m Manifest) archivePath(component string) string {
	return filepath.Join(m.Dir, component+".tar")
}

// Cross references the components included in indexes with the components being pushed.
func (c RuntimeConfig) validate() error {
	var errs []error
	for _, tagFragment := range c.tagFragments {
		pushedImages := c.manifest.match(tagFragment)
		for _, imageIndex := range c.indexTags {
			for _, comp := range imageIndex.Components {
				if imageTags, ok := pushedImages[comp]; !ok || len(imageTags) == 0 {
					errs = append(errs, fmt.Errorf(
						"index with tags [%s] references component %s not pushed under tag fragment %s",
						strings.Join(imageIndex.Tags, ", "),
						comp,
						tagFragment))
				}
			}
		}
	}
	return errors.Join(errs...)
}

// Write the exports data to the configured output path, returning the path to the file written.
func (c *RuntimeConfig) writeExports() (string, error) {
	exportPath := c.manifest.Export
	pathname := filepath.Join(
		exportPath,
		strings.NewReplacer("/", "_", "\\", "_", ".", "_").Replace(c.manifestPath+"-"+c.repo+"-"+strings.Join(c.tagFragments, "-"))+".json",
	)
	exported, err := json.Marshal(c.exports)
	if err != nil {
		return "", fmt.Errorf("marshaling json: %w", err)
	}

	err = os.MkdirAll(exportPath, 0o777)
	if err != nil {
		return "", fmt.Errorf("making dest dir: %w", err)
	}

	if err := os.WriteFile(pathname, exported, 0o600); err != nil {
		return "", fmt.Errorf("writing export: %w", err)
	}

	return pathname, nil
}
