package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v2"
)

type ReleaseMetadata struct {
	Major                   int
	Minor                   int
	ReleaseManagerHandle    string
	ReleaseBlogPostMR       int
	ReleaseBlogPostDeadline string
	HelmChartMajor          int
	HelmChartMinor          int
	HelmChartPatch          int
}

const (
	GitLabRunnerProjectID          = "gitlab-org/gitlab-runner"
	GitLabRunnerHelmChartProjectID = "charts/gitlab-runner"
	WWWGitlabComProjectID          = "gitlab-com/www-gitlab-com"

	ReleasePostLabel = "release post"

	ReleaseManagerHandleEnvVariable = "GITLAB_RUNNER_RELEASE_MANAGER_HANDLE"

	VersionFile          = "./VERSION"
	HelmChartVersionFile = "Chart.yaml"

	LayoutDay = "2006-01-02"
)

var (
	reader          *bufio.Reader
	releaseMetadata ReleaseMetadata

	detectedVersion          []string
	detectedMergeRequest     []string
	detectedHelmChartVersion []string

	templateFilePath = flag.String("issue-template-file", ".gitlab/issue_templates/Release Checklist.md", "Path to a file with issue template")

	dryRun        = flag.Bool("dry-run", false, "Show issue content instead of creating it in GitLab")
	noInteractive = flag.Bool("no-interactive", false, "Don't ask, just try to work!")

	major                   = flag.String("major", detectVersion()[0], "Major version number")
	minor                   = flag.String("minor", detectVersion()[1], "Minor version number")
	releaseManagerHandle    = flag.String("release-manager-handle", defaultReleaseManagerHandle(), "GitLab.com handle of the release manager")
	releaseBlogPostMR       = flag.String("release-blog-post-mr", detectBlogPostMergeRequest()[0], "ID of the Release Blog Post MR")
	releaseBlogPostDeadline = flag.String("release-blog-post-deadline", detectReleaseMergeRequestDeadline(), "Deadline for adding Runner specific content to the Release Blog Post")

	helmChartMajor = flag.String("helm-chart-major", detectHelmChartVersion()[0], "Major version number of GitLab Runner Helm Chart")
	helmChartMinor = flag.String("helm-chart-minor", detectHelmChartVersion()[1], "Minor version number of GitLab Runner Helm Chart")
	helmChartPatch = flag.String("helm-chart-patch", detectHelmChartVersion()[2], "Patch version number of GitLab Runner Helm Chart")
)

func detectVersion() []string {
	if len(detectedVersion) > 0 {
		return detectedVersion
	}

	fmt.Println("Auto-detecting version...")

	content, err := ioutil.ReadFile(VersionFile)
	if err != nil {
		fmt.Printf("Error while reading version file %q: %v", VersionFile, err)

		return []string{"", ""}
	}

	fmt.Printf("Found: %s\n", content)

	detectedVersion = strings.Split(string(content), ".")

	return detectedVersion
}

type HelmChartData struct {
	Version string `yaml:"version"`
}

func detectHelmChartVersion() []string {
	if len(detectedHelmChartVersion) > 0 {
		return detectedHelmChartVersion
	}

	fmt.Println("Auto-detecting Helm Chart version...")

	charVersionFileURL := fmt.Sprintf("https://gitlab.com/%s/raw/master/%s", GitLabRunnerHelmChartProjectID, HelmChartVersionFile)
	req, err := http.NewRequest(http.MethodGet, charVersionFileURL, nil)
	if err != nil {
		panic(fmt.Errorf("error while creating helm chart version detection request: %v", err))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(fmt.Errorf("error while requesting helm chart version: %v", err))
	}

	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Errorf("error while reading helm chart version response: %v", err))
	}

	helmChartData := new(HelmChartData)
	err = yaml.Unmarshal(content, helmChartData)
	if err != nil {
		fmt.Printf("Error while parsing Helm Chart version file: %v", err)

		return []string{"", "", ""}
	}

	fmt.Printf("Found: %s\n", helmChartData.Version)

	versionRx := regexp.MustCompile(`([0-9]+)\.([0-9]+)\.([0-9]+)`)
	versions := versionRx.FindAllStringSubmatch(helmChartData.Version, -1)

	if len(versions) != 1 {
		fmt.Printf("Couldn't parse the version string")

		return []string{"", "", ""}
	}

	detectedHelmChartVersion = []string{
		versions[0][1],
		versions[0][2],
		versions[0][3],
	}

	return detectedHelmChartVersion
}

func defaultReleaseManagerHandle() string {
	fmt.Println("Auto-detecting Release Manager handle...")

	handle := os.Getenv(ReleaseManagerHandleEnvVariable)
	fmt.Printf("Found: %s\n", handle)

	return handle
}

type listMergeRequestsResponse []listMergeRequestsResponseEntry

type listMergeRequestsResponseEntry struct {
	ID          int    `json:"iid"`
	WebURL      string `json:"web_url"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func detectBlogPostMergeRequest() []string {
	if len(detectedMergeRequest) > 0 {
		return detectedMergeRequest
	}

	fmt.Println("Auto-detecting Release Post merge request...")

	version := detectVersion()

	q := url.Values{}
	q.Add("labels", ReleasePostLabel)
	q.Add("state", "opened")
	q.Add("milestone", fmt.Sprintf("%s.%s", version[0], version[1]))

	rawURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/merge_requests?%s", url.QueryEscape(WWWGitlabComProjectID), q.Encode())

	findMergeRequestURL, err := url.Parse(rawURL)
	if err != nil {
		fmt.Printf("Error while parsing findMergeRequestURL: %v", err)

		return []string{"", ""}
	}

	req, err := http.NewRequest(http.MethodGet, findMergeRequestURL.String(), nil)
	if err != nil {
		fmt.Printf("Error while creating HTTP Request: %v", err)

		return []string{"", ""}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error while requesting API endpoint: %v", err)

		return []string{"", ""}
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error while reading response body: %v", err)

		return []string{"", ""}
	}

	var response listMergeRequestsResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Printf("Error while parsing response JSON: %v", err)

		return []string{"", ""}
	}

	printEntry := func(entry listMergeRequestsResponseEntry) {
		fmt.Printf("\t%-40q %s\n", entry.Title, entry.WebURL)
	}

	fmt.Println("Found following www-gitlab-com merge requests:")
	for _, entry := range response {
		printEntry(entry)
	}

	if len(response) < 1 {
		fmt.Println("Release Post merge request was not auto-detected. Please enter the ID manually")

		return []string{"", ""}
	}

	chosen := response[0]

	fmt.Println("Choosing:")
	printEntry(chosen)

	r := regexp.MustCompile("gitlab.com/gitlab-com/www-gitlab-com/blob/release-\\d+-\\d+/data/release_posts/(\\d+)_(\\d+)_(\\d+)_gitlab_\\d+_\\d+_released.yml")
	dateParts := r.FindStringSubmatch(chosen.Description)

	detectedMergeRequest = []string{
		strconv.Itoa(chosen.ID),
		fmt.Sprintf("%s-%s-%s", dateParts[1], dateParts[2], dateParts[3]),
	}

	return detectedMergeRequest
}

func detectReleaseMergeRequestDeadline() string {
	fmt.Println("Auto-detecting Release Post entry deadline...")

	offsetMap := map[time.Weekday]int{
		time.Monday:    -11,
		time.Tuesday:   -11,
		time.Wednesday: -9,
		time.Thursday:  -9,
		time.Friday:    -9,
		time.Saturday:  -9,
		time.Sunday:    -10,
	}

	date := detectBlogPostMergeRequest()[1]
	if len(date) < 1 {
		fmt.Println("Could not detect the date of Release...")

		return ""
	}

	releaseDate, err := time.Parse(LayoutDay, date)
	if err != nil {
		fmt.Printf("Could not parse detected date %q: %v", date, err)

		return ""
	}

	offset := offsetMap[releaseDate.Weekday()]

	deadlineTime := releaseDate.Add(time.Duration(24*offset) * time.Hour)
	deadline := deadlineTime.Format(LayoutDay)

	fmt.Printf("Decided to use %q. Please adjust if required!\n", deadline)

	return deadline
}

func main() {
	fmt.Println()
	fmt.Println("\nGitLab Runner release checklist issue generator")
	fmt.Println()

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n  %s [OPTIONS]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	prepareMetadata()

	content := prepareIssueContent()
	title := prepareIssueTitle()

	if *dryRun {
		printIssue(title, content)
	} else {
		postIssue(title, content)
	}
}

func prepareMetadata() {
	var err error

	askOnce("Major version number", major)
	releaseMetadata.Major, err = strconv.Atoi(*major)
	if err != nil {
		panic(err)
	}

	askOnce("Minor version number", minor)
	releaseMetadata.Minor, err = strconv.Atoi(*minor)
	if err != nil {
		panic(err)
	}

	askOnce("Helm Chart Major version number", helmChartMajor)
	releaseMetadata.HelmChartMajor, err = strconv.Atoi(*helmChartMajor)
	if err != nil {
		panic(err)
	}

	askOnce("Helm Chart Minor version number", helmChartMinor)
	releaseMetadata.HelmChartMinor, err = strconv.Atoi(*helmChartMinor)
	if err != nil {
		panic(err)
	}

	askOnce("Helm Chart Patch version number", helmChartPatch)
	releaseMetadata.HelmChartPatch, err = strconv.Atoi(*helmChartPatch)
	if err != nil {
		panic(err)
	}

	askOnce("GitLab.com handle of the release manager", releaseManagerHandle)
	releaseMetadata.ReleaseManagerHandle = *releaseManagerHandle

	askOnce("ID of the Release Blog Post MR", releaseBlogPostMR)
	releaseMetadata.ReleaseBlogPostMR, err = strconv.Atoi(*releaseBlogPostMR)
	if err != nil {
		panic(err)
	}

	askOnce("Deadline for adding Runner specific content to the Release Blog Post", releaseBlogPostDeadline)
	releaseMetadata.ReleaseBlogPostDeadline = *releaseBlogPostDeadline
}

func askOnce(prompt string, result *string) {
	fmt.Printf("%s [%s]: ", prompt, *result)
	if *noInteractive {
		fmt.Println()

		return
	}

	if reader == nil {
		reader = bufio.NewReader(os.Stdin)
	}

	data, _, err := reader.ReadLine()
	if err != nil {
		panic(err)
	}

	newResult := string(data)
	newResult = strings.TrimSpace(newResult)

	if newResult != "" {
		*result = newResult
	}

	if *result == "" {
		panic("Can't be left empty!")
	}
}

func prepareIssueContent() string {
	data, err := ioutil.ReadFile(*templateFilePath)
	if err != nil {
		panic(err)
	}

	tpl := template.New("release-issue")
	tpl.Funcs(template.FuncMap{
		"inc": func(i int) int {
			return i + 1
		},
	})

	tpl, err = tpl.Parse(string(data))
	if err != nil {
		panic(err)
	}

	var output []byte
	buffer := bytes.NewBuffer(output)
	err = tpl.Execute(buffer, releaseMetadata)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

func prepareIssueTitle() string {
	return fmt.Sprintf("GitLab Runner %d.%d release checklist", releaseMetadata.Major, releaseMetadata.Minor)
}

func printIssue(title, content string) {
	fmt.Println()
	fmt.Println("====================================")
	fmt.Printf("  Title: %s\n", title)
	fmt.Printf("Content:\n\n%s\n", content)
	fmt.Println("====================================")
	fmt.Println()
}

type createIssueOptions struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type createIssueResponse struct {
	WebURL string `json:"web_url"`
}

func postIssue(title, content string) {
	newIssueURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/issues", url.QueryEscape(GitLabRunnerProjectID))

	options := &createIssueOptions{
		Title:       title,
		Description: content,
	}

	jsonBody, err := json.Marshal(options)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(http.MethodPost, newIssueURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		panic(err)
	}

	token := os.Getenv("GITLAB_API_PRIVATE_TOKEN")
	req.Header.Set("Private-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var response createIssueResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Created new issue: %s", response.WebURL)
}
