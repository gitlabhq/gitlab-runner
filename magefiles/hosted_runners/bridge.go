package hosted_runners

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/magefile/mage/sh"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docutils"
)

var (
	preVersionRx = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+~pre.[0-9]+.g[a-f0-9]+$`)

	errNothingToUpdate = errors.New("nothing to update")
)

type BridgeInfo struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
	Flavor    string `json:"flavor"`
	Timestamp string `json:"timestamp"`
}

func Bridge(ctx context.Context, log *slog.Logger, wikiClient *GitLabWikiClient) error {
	info, err := prepareBridgeInfo(log)
	if err != nil {
		return fmt.Errorf("preparing bridge info: %w", err)
	}

	wikiPage, err := wikiClient.Read(ctx)
	if err != nil {
		return fmt.Errorf("reading Wiki page: %w", err)
	}

	replacer := docutils.NewSectionReplacerWithLogger(log, "runner_version_table", bytes.NewBufferString(wikiPage.Content))
	err = replacer.Replace(prepareReplaceFn(log, info))
	if err != nil {
		if errors.Is(err, errNothingToUpdate) {
			log.Info("No changes to update")
			return nil
		}

		return fmt.Errorf("rewriting runner_version_table section: %w", err)
	}

	err = wikiClient.Update(ctx, WikiPage{Content: replacer.Output()})
	if err != nil {
		return fmt.Errorf("updating Wiki page: %w", err)
	}

	log.Info("Version list updated")

	return nil
}

func prepareBridgeInfo(log *slog.Logger) (BridgeInfo, error) {
	var info BridgeInfo

	wd, err := os.Getwd()
	if err != nil {
		return info, fmt.Errorf("retrieving current working directory: %w", err)
	}

	versionScript := filepath.Join(wd, "ci", "version")

	version, err := sh.Output(versionScript)
	if err != nil {
		return info, fmt.Errorf("computing runner version: %w", err)
	}

	flavor := "tagged"
	if preVersionRx.MatchString(version) {
		flavor = "pre"
	}
	log.Info("Runner version", "version", version, "flavor", flavor)

	commitSHA := os.Getenv("CI_COMMIT_SHA")
	log.Info("Runner commit SHA", "SHA", commitSHA)

	info = BridgeInfo{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   version,
		CommitSHA: commitSHA,
		Flavor:    flavor,
	}

	return info, nil
}

func prepareReplaceFn(log *slog.Logger, info BridgeInfo) docutils.SectionReplacerFN {
	return func(in io.Reader) (string, error) {
		inputBytes, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("reading input: %w", err)
		}

		input := string(inputBytes)
		log.Debug("Original input", "input", input)

		input = strings.ReplaceAll(input, "```json:table", "")
		input = strings.ReplaceAll(input, "```", "")

		log.Debug("Processed input", "input", input)

		inBuf := bytes.NewBufferString(input)

		var v WikiJSONTable

		decoder := json.NewDecoder(inBuf)
		err = decoder.Decode(&v)
		log.Debug("Decoding input", "error", err)

		if err != nil {
			return "", fmt.Errorf("decoding Wiki JSON table: %w", err)
		}

		for _, item := range v.Items {
			log.Debug("Processing item", "item", item, "should-do-nothing", item.Version == info.Version)
			if item.Version == info.Version {
				log.Info("Version already exists on the list; skipping", "version", info.Version)

				return "", errNothingToUpdate
			}
		}

		v.Items = append([]BridgeInfo{info}, v.Items...)

		outBuf := new(bytes.Buffer)

		encoder := json.NewEncoder(outBuf)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(v)

		log.Debug("Encoding output", "error", err)

		if err != nil {
			return "", fmt.Errorf("encoding Wiki JSON table: %w", err)
		}

		output := fmt.Sprintf("```json:table\n%s\n```\n", outBuf.String())

		log.Debug("Prepared output", "output", output)

		return output, nil
	}
}
