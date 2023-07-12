// packagecloud-releases.go prints the OS distribution/version combinations
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
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiEndpoint = "/api/v1/distributions.json"
	usage       = "Usage:\n\tpackagecloud-releases <rpm|deb> <stable|bleeding>"
)

type (
	pkgCloudDistroRelease struct {
		IndexName string `json:"index_name"`
	}

	pkgCloudDistribution struct {
		IndexName string                  `json:"index_name"`
		Versions  []pkgCloudDistroRelease `json:"versions"`
	}

	pkgCloudDistributionsResult map[string][]pkgCloudDistribution

	envArgs struct {
		token string
		host  string
	}

	cmdArgs struct {
		branch string
		pkg    string
	}
)

var (
	supportedDistros = map[string][]string{
		"deb/stable":   {"debian", "ubuntu", "raspbian", "linuxmint"},
		"deb/bleeding": {"debian", "ubuntu"},
		"rpm/stable":   {"fedora", "ol", "el", "amazon"},
		"rpm/bleeding": {"fedora", "el", "amazon"},
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
	}
)

func (ca *cmdArgs) String() string {
	return ca.pkg + "/" + ca.branch
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	args, err := getCmdArgs(os.Args)
	if err != nil {
		return err
	}

	env, err := getEnvArgs()
	if err != nil {
		return err
	}

	allDistroReleases, err := getPkgCloudDistrosReleases(env.token, env.host)
	if err != nil {
		return err
	}

	distrosToPackage := supportedDistros[args.String()]

	versionsToPackage := getDistroReleasesToPackage(distrosToPackage, allDistroReleases[args.pkg])
	fmt.Println(strings.Join(versionsToPackage, " "))
	return nil
}

// getEnvArgs ensures the required environment variables exist, but does not
// attempt to validate them.
func getEnvArgs() (envArgs, error) {
	env := envArgs{
		token: os.Getenv("PACKAGECLOUD_TOKEN"),
		host:  os.Getenv("PACKAGE_CLOUD_URL"),
	}

	if env.token == "" || env.host == "" {
		return envArgs{}, fmt.Errorf("missing 'PACKAGE_CLOUD_URL' and/or 'PACKAGECLOUD_TOKEN'")
	}

	return env, nil
}

// getCmd ensures the required command line arguments have been specified and
// are valid.
func getCmdArgs(osArgs []string) (cmdArgs, error) {
	if len(osArgs) != 3 {
		return cmdArgs{}, fmt.Errorf("missing package type and/or branch: %q\n%s", strings.Join(osArgs, " "), usage)
	}

	args := cmdArgs{
		pkg:    osArgs[1],
		branch: osArgs[2],
	}

	if _, ok := supportedDistros[args.String()]; !ok {
		return cmdArgs{}, fmt.Errorf("no supported distros for package %q and branch %q", args.pkg, args.branch)
	}

	return args, nil
}

func reverse[S ~[]E, E any](s S) S {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func contains[S ~[]E, E comparable](s S, e E) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
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
		if !contains(supportedDistros, distro.IndexName) {
			continue
		}

		for _, version := range reverse(distro.Versions) {
			if !contains(skipReleases[distro.IndexName], version.IndexName) {
				versionToPackage = append(versionToPackage, distro.IndexName+"/"+version.IndexName)
			}
			if oldestRelease[distro.IndexName] == version.IndexName {
				break
			}
		}
	}

	return versionToPackage
}

// getPkgCloudDistrosReleases queries the packagecloud API to get a list of ALL
// the OS distro/releases it supports. The JSON response surtructure is as follows:
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
	req, err := http.NewRequest("GET", host+apiEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for url %q: %w", host+apiEndpoint, err)
	}
	req.SetBasicAuth(token, "")

	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get url %q: %w", host+apiEndpoint, err)
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
