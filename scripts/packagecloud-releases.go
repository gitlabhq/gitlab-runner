// Return the <maxVersionsPerDistro> latest releases for each of the input distros.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const apiEndpoint = "/api/v1/distributions.json"

type (
	SupportedDistros map[string]struct{}
	DistroVersion    struct {
		DisplayName   string `json:"display_name"`
		IndexName     string `json:"index_name"`
		VersionNumber string `json:"version_number"`
		ID            int    `json:"id"`
	}
	Distro struct {
		DisplayName string          `json:"display_name"`
		IndexName   string          `json:"index_name"`
		Versions    []DistroVersion `json:"versions"`
	}
	Result map[string][]Distro
)

var supportedPackageTypes = [...]string{"deb", "rpm"}

type envArgs struct {
	token             string
	host              string
	releasesPerDistro int
}

func exitIfErr(e error) {
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}
}

func getEnvArgs() (envArgs, error) {
	var err error
	env := envArgs{}
	env.token = os.Getenv("PACKAGECLOUD_TOKEN")
	env.host = os.Getenv("PACKAGE_CLOUD_URL")
	env.releasesPerDistro, err = strconv.Atoi(os.Getenv("NUM_DISTRO_RELEASES"))

	if err != nil {
		return envArgs{}, fmt.Errorf("bad or missing 'NUM_DISTRO_RELEASES': %w", err)
	}

	if env.token == "" || env.host == "" {
		return envArgs{}, fmt.Errorf("missing 'PACKAGE_CLOUD_URL' and/or 'PACKAGECLOUD_TOKEN'")
	}

	return env, nil
}

func main() {
	env, err := getEnvArgs()
	exitIfErr(err)

	supportedDistros := normalizeInput(os.Args[1:])
	if len(supportedDistros) == 0 {
		exitIfErr(fmt.Errorf("no supported distributions specified %q", strings.Join(os.Args[1:], " ")))
	}

	allDistroReleases, err := getData(env.token, env.host)
	exitIfErr(err)

	versionsToPackage := getDistroReleasesToPackage(supportedDistros, env.releasesPerDistro, allDistroReleases)
	fmt.Println(strings.Join(versionsToPackage, " "))
}

// Depending on shell quoting, the command could receive many args with one
// entry per arg, or a single arg with all entries as a single arg.
// Normalize/flatten the input, and put it in a map for faster/easier lookups.
func normalizeInput(input []string) SupportedDistros {
	distros := SupportedDistros{}
	for i := range input {
		for _, distro := range strings.Fields(input[i]) {
			distros[distro] = struct{}{}
		}
	}
	return distros
}

// Return a list of the latest/newest <maxVersionsPerDistro> releases supported
// by packagecloud for each distro in <supportedDistros>.
func getDistroReleasesToPackage(supportedDistros SupportedDistros, releasesPerDistro int, data Result) []string {
	result := []string{}
	for _, pkg := range supportedPackageTypes {
		for _, distro := range data[pkg] {
			if _, ok := supportedDistros[distro.IndexName]; !ok {
				continue
			}

			for i, n := len(distro.Versions)-1, 0; i >= 0 && n < releasesPerDistro; i, n = i-1, n+1 {
				result = append(result, distro.IndexName+"/"+distro.Versions[i].IndexName)
			}
		}
	}

	return result
}

// Query the packagecloud API to get a list of all the os/distro/releases it
// supports.
func getData(token, host string) (Result, error) {
	basicAuth := base64.StdEncoding.EncodeToString([]byte(token + ":"))
	result := Result{}

	req, err := http.NewRequest("GET", host+apiEndpoint, nil)
	if err != nil {
		return result, err
	}
	req.Header.Add("Authorization", "Basic "+basicAuth)

	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("got unexpected response status code: " + resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}
