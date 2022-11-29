// Return the <maxVersionsPerDistro> latest releases for each of the input distros.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

var (
	supportedPackageTypes = []string{"deb", "rpm"}
	token                 string
	host                  string
	releasesPerDistro     int
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}

func init() {
	getenv := func(name string) string {
		value := os.Getenv(name)
		if value == "" {
			panic(name + " environment variable not defined")
		}
		return value
	}

	token = getenv("PACKAGECLOUD_TOKEN")
	host = getenv("PACKAGE_CLOUD_URL")
	var err error
	releasesPerDistro, err = strconv.Atoi(getenv("NUM_DISTRO_RELEASES"))
	must(err)
}

func main() {
	supportedDistros := normalizeInput(os.Args[1:])
	if len(supportedDistros) == 0 {
		log.Fatalf("no supported distributions specified %q", strings.Join(os.Args[1:], " "))
	}
	versionsToPackage := getDistroVersionsToPackage(supportedDistros, getData())
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
func getDistroVersionsToPackage(supportedDistros SupportedDistros, data Result) []string {
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
func getData() Result {
	basicAuth := base64.StdEncoding.EncodeToString([]byte(token + ":"))

	url := host + apiEndpoint

	req, err := http.NewRequest("GET", url, nil)
	must(err)
	req.Header.Add("Authorization", "Basic "+basicAuth)
	resp, err := http.DefaultClient.Do(req)
	must(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		panic("got unexpected response status code: " + resp.Status)
	}

	d := json.NewDecoder(resp.Body)
	result := Result{}
	err = d.Decode(&result)
	must(err)

	return result
}
