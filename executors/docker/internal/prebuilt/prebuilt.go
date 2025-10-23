package prebuilt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/homedir"
)

const (
	prebuiltExportImageExtension        = ".tar.xz"
	prebuiltDockerArchiveImageExtension = ".docker.tar.zst"
)

var PrebuiltImagesPaths []string

func init() {
	runner, err := os.Executable()
	if err != nil {
		logrus.Errorln(
			"Docker executor: unable to detect gitlab-runner folder, "+
				"prebuilt image helpers will be loaded from remote registry.",
			err,
		)
	}

	runnerFolder := filepath.Dir(runner)

	PrebuiltImagesPaths = []string{
		// When gitlab-runner is running from repository root
		filepath.Join(runnerFolder, "out/helper-images"),
		// When gitlab-runner is running from `out/binaries`
		filepath.Join(runnerFolder, "../helper-images"),
		// Add working directory path, used when running from temp directory, such as with `go run`
		filepath.Join(homedir.New().GetWDOrEmpty(), "out/helper-images"),
	}
	if runtime.GOOS == "linux" {
		// This section covers the Linux packaged app scenario, with the binary in /usr/bin.
		// The helper images are located in /usr/lib/gitlab-runner/helper-images,
		// as part of the packaging done in the create_package function in ci/package
		PrebuiltImagesPaths = append(
			PrebuiltImagesPaths,
			filepath.Join(runnerFolder, "../lib/gitlab-runner/helper-images"),
		)
	}
}

func Get(ctx context.Context, client docker.Client, info helperimage.Info) (*image.InspectResponse, error) {
	if err := load(ctx, client, info); err != nil {
		return nil, err
	}

	image, _, err := client.ImageInspectWithRaw(ctx, info.String())
	if err == nil {
		return &image, nil
	}

	return nil, err
}

func load(ctx context.Context, client docker.Client, info helperimage.Info) error {
	imagePaths := []string{
		info.Prebuilt + prebuiltDockerArchiveImageExtension,
		info.Prebuilt + prebuiltExportImageExtension,
	}

	// future proof using amd64 in the future over x86_64
	if strings.Contains(info.Prebuilt, "x86_64") {
		name := strings.ReplaceAll(info.Prebuilt, "x86_64", "amd64")
		imagePaths = append(
			imagePaths,
			name+prebuiltDockerArchiveImageExtension,
			name+prebuiltExportImageExtension,
		)
	}

	var errs []error
	for _, imageDir := range PrebuiltImagesPaths {
		for _, imagePath := range imagePaths {
			importPath := filepath.Join(imageDir, imagePath)

			if strings.HasSuffix(imagePath, prebuiltDockerArchiveImageExtension) {
				if err := imageLoad(ctx, client, importPath, info.Name, info.Tag); err != nil {
					errs = append(errs, fmt.Errorf("loading %v: %w", imagePath, err))
					continue
				}

				return nil
			}

			if err := imageImport(ctx, client, importPath, info.Name, info.Tag); err != nil {
				errs = append(errs, fmt.Errorf("importing %v: %w", imagePath, err))
				continue
			}

			return nil
		}
	}

	return errors.Join(errs...)
}

func imageLoad(ctx context.Context, client docker.Client, path, ref, tag string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	resp, err := client.ImageLoad(ctx, file, true)
	if err != nil {
		return fmt.Errorf("failed to load image: %w", err)
	}
	defer resp.Body.Close()
	defer func() { _, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024)) }()

	// image load makes it unnecessarily difficult to get the image ref
	var event struct {
		Stream string `json:"stream"`
	}

	decoder := json.NewDecoder(resp.Body)

	var imageID string
	for decoder.More() {
		if err := decoder.Decode(&event); err != nil {
			return fmt.Errorf("decoding image id: %w", err)
		}

		switch {
		case strings.Contains(event.Stream, "Loaded image:"):
			imageID = strings.TrimSpace(strings.TrimPrefix(event.Stream, "Loaded image:"))
		case strings.Contains(event.Stream, "Loaded image ID:"):
			imageID = strings.TrimSpace(strings.TrimPrefix(event.Stream, "Loaded image ID:"))
		}

		if imageID != "" {
			break
		}
	}

	if imageID == "" {
		return fmt.Errorf("could not find image ID for loaded prebuilt image")
	}

	if err := client.ImageTag(ctx, imageID, ref+":"+tag); err != nil {
		return fmt.Errorf("tagging %v to %v:%v", imageID, ref, tag)
	}

	return nil
}

func imageImport(ctx context.Context, client docker.Client, path, ref, tag string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	source := image.ImportSource{
		Source:     file,
		SourceName: "-",
	}
	options := image.ImportOptions{
		Tag: tag,
		// NOTE: The ENTRYPOINT metadata is not preserved on export, so we need to reapply this metadata on import.
		// See https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2058#note_388341301
		Changes: []string{`ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]`},
	}

	if err = client.ImageImportBlocking(ctx, source, ref, options); err != nil {
		return fmt.Errorf("failed to import image: %w", err)
	}

	return nil
}
