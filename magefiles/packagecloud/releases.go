// Package packagecloud prints the OS distribution/version combinations
// supported by packagecloud for which we want to publish gitlab-runner
// packages, for the specified package type (deb|rpm) and branch
// (stable|bleeding). This will be a subset of all the distro versions
// supported by packagecloud as follows:
//   - Only distributions for the specified package type (deb or rpm)
//   - Only distributions for the specified branch (stable or bleeding)
//   - Only distributions for the supported distros (supportedDistrosByPackageAndBranch)
//   - Only releases not mentioned in skipReleases
//   - Only releases up to and including oldestRelease (i.e. releases older
//     than oldestRelease will be excluded).
//
// The resulting list will be formatted as 'distro/release` for each supported
// distro/release' combination.
//
// Making changes:
// The list of releases for each distro can be modified by adding the
// appropriate entries into `oldestRelease` and/or `skipReleases`:
// - `skipReleases` can be used to ommit releases NEWER than the oldest release
// for a distro (e.g. non LST releases in Ubuntu).
// - `oldestRelease` will set the oldest relase to include for a distro.
// Releases older than the ones mentioned here will be excluded.
// - New distros for a package type of branch can be added by adding entries
// into `supportedDistrosByPackageAndBranch`, `oldestRelease`, and
// `skipReleases`.
package packagecloud

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/samber/lo"
)

const (
	distributionsListAPIEndpoint = "/api/v1/distributions.json"
	usage                        = "Usage:\n\tpackagecloud-releases <rpm|deb> <stable|bleeding>"
)

type (
	pkgCloudDistroRelease struct {
		IndexName string `json:"index_name"`
		ID        int    `json:"id"`
	}

	pkgCloudDistribution struct {
		IndexName string                  `json:"index_name"`
		Versions  []pkgCloudDistroRelease `json:"versions"`
	}

	pkgCloudDistributionsResult map[string][]pkgCloudDistribution

	args struct {
		branch string
		dist   string
		token  string
		host   string
	}
)

var (
	supportedDistros = map[string][]string{
		"deb/stable":   {"debian", "ubuntu", "raspbian", "linuxmint"},
		"deb/bleeding": {"debian", "ubuntu"},
		"rpm/stable":   {"fedora", "ol", "el", "amazon", "sles", "opensuse"},
		"rpm/bleeding": {"fedora", "el", "amazon", "sles", "opensuse"},
	}

	oldestRelease = map[string]string{
		"debian":    "stretch",
		"ubuntu":    "xenial",
		"raspbian":  "jessie",
		"linuxmint": "sarah",
		"fedora":    "32",
		"ol":        "6",
		"el":        "7",
		"amazon":    "2",
		"sles":      "12.3",
		"opensuse":  "42.3",
	}

	skipReleases = map[string][]string{
		"debian":    {},
		"ubuntu":    {"hirsute", "groovy", "eoan", "disco", "cosmic", "artful", "zesty", "yakkety"},
		"raspbian":  {},
		"linuxmint": {"sylvia", "tara", "tessa", "tina", "tricia"},
		"fedora":    {},
		"ol":        {},
		"el":        {},
		"amazon":    {},
		"sles":      {},
		"opensuse":  {},
	}
)

func (a *args) DistBranchPair() string {
	return a.dist + "/" + a.branch
}

func Releases(dist, branch, token, host string) ([]string, error) {
	args := args{
		branch: branch,
		dist:   dist,
		token:  token,
		host:   host,
	}

	if _, ok := supportedDistros[args.DistBranchPair()]; !ok {
		return nil, fmt.Errorf("no supported distros for package %q and branch %q", args.dist, args.branch)
	}

	allDistroReleases, err := getPkgCloudDistrosReleases(args.token, args.host)
	if err != nil {
		return nil, err
	}

	distrosToPackage := supportedDistros[args.DistBranchPair()]

	versionsToPackage := getDistroReleasesToPackage(distrosToPackage, allDistroReleases[args.dist])
	return versionsToPackage, nil
}

// getDistroReleasesToPackage returns the subset of pkgCloudDistributionsResult
// for which we want to publish gitlab-runner packages. The subset will be only
// distro and version:
//   - for the specified package type (deb or rpm)
//   - for the specified branch (stable or bleeding)
//   - for the supported distros (supportedDistrosByPackageAndBranch)
//   - for releases not mentioned in skipReleases
//   - releases up to and including oldestRelease (i.e. releases older than
//     oldestRelease will be excluded).
//
// The resulting list will be formatted as 'distro/release` for each supported
// distro/release' combination.
func getDistroReleasesToPackage(supportedDistros []string, pkgCloudDistros []pkgCloudDistribution) []string {
	var versionToPackage []string

	for _, distro := range pkgCloudDistros {
		if !lo.Contains(supportedDistros, distro.IndexName) {
			continue
		}

		slices.SortFunc(distro.Versions, func(a, b pkgCloudDistroRelease) int {
			return cmp.Compare(b.ID, a.ID)
		})

		for _, version := range distro.Versions {
			if !lo.Contains(skipReleases[distro.IndexName], version.IndexName) {
				versionToPackage = append(versionToPackage, distro.IndexName+"/"+version.IndexName)
			}
			if oldestRelease[distro.IndexName] == version.IndexName {
				break
			}
		}
	}

	return versionToPackage
}

// getPkgCloudDistrosReleases queries the PackageCloud API to get a list of ALL
// the OS distro/releases it supports. The JSON response structure is as follows:
//
//	{
//	  "deb": [
//	    {
//	      "display_name": "Ubuntu",
//	      "index_name": "ubuntu",
//	      "versions": [
//	        {
//	          "id": 2,
//	          "display_name": "4.10 Warty Warthog",
//	          "index_name": "warty",
//	          "version_number": "4.10"
//	        },
//	        ...
//	      ]
//	    },
//	    {
//	      "display_name": "Debian",
//	      "index_name": "debian",
//	      "versions": [
//	        {
//	          "id": 23,
//	          "display_name": "4.0 etch",
//	          "index_name": "etch",
//	          "version_number": "4.0"
//	        },
//	        ...
//	      ]
//	    },
//	    ...
//	  ],
//	  "rpm": [
//	    {
//	      "display_name": "Fedora",
//	      "index_name": "fedora",
//	      "versions": [
//	        ...
//	        {
//	          "id": 140,
//	          "display_name": "36 Fedora 36",
//	          "index_name": "36",
//	          "version_number": "36.0"
//	        }
//	      ]
//	    },
//	    ...
//	  ],
//	}
func getPkgCloudDistrosReleases(token, host string) (pkgCloudDistributionsResult, error) {
	apiURL, err := url.JoinPath(host, distributionsListAPIEndpoint)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for url %q: %w", apiURL, err)
	}
	req.SetBasicAuth(token, "")

	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get url %q: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got unexpected response status code: %s", resp.Status)
	}

	result := pkgCloudDistributionsResult{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}
	return result, nil
}
