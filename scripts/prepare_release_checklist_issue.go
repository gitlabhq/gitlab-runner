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
	"strconv"
	"strings"
	"text/template"
)

type ReleaseMetadata struct {
	Major                   int
	Minor                   int
	ReleaseManagerHandle    string
	ReleaseBlogPostMR       int
	ReleaseBlogPostDeadline string
}

const GITLAB_RUNNER_PROJECT_ID = "gitlab-org/gitlab-runner"

var reader *bufio.Reader
var releaseMetadata ReleaseMetadata

var templateFilePath = flag.String("issue-template-file", ".gitlab/issue_templates/Release Checklist.md", "Path to a file with issue template")

var dryRun = flag.Bool("dry-run", false, "Show issue content instead of creating it in GitLab")
var noInteractive = flag.Bool("no-interactive", false, "Don't ask, just try to work!")

var major = flag.String("major", "", "Major version number")
var minor = flag.String("minor", "", "Minor version number")
var releaseManagerHandle = flag.String("release-manager-handle", "", "GitLab.com handle of the release manager")
var releaseBlogPostMR = flag.String("release-blog-post-mr", "", "ID of the Release Blog Post MR")
var releaseBlogPostDeadline = flag.String("release-blog-post-deadline", "", "Deadline for adding Runner specific content to the Release Blog Post")

func main() {
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

	askOnce("Major version number:", major)
	releaseMetadata.Major, err = strconv.Atoi(*major)
	if err != nil {
		panic(err)
	}

	askOnce("Minor version number:", minor)
	releaseMetadata.Minor, err = strconv.Atoi(*minor)
	if err != nil {
		panic(err)
	}

	askOnce("GitLab.com handle of the release manager:", releaseManagerHandle)
	releaseMetadata.ReleaseManagerHandle = *releaseManagerHandle

	askOnce("ID of the Release Blog Post MR:", releaseBlogPostMR)
	releaseMetadata.ReleaseBlogPostMR, err = strconv.Atoi(*releaseBlogPostMR)
	if err != nil {
		panic(err)
	}

	askOnce("Deadline for adding Runner specific content to the Release Blog Post:", releaseBlogPostDeadline)
	releaseMetadata.ReleaseBlogPostDeadline = *releaseBlogPostDeadline
}

func askOnce(prompt string, result *string) {
	if *noInteractive {
		return
	}

	fmt.Println(prompt)

	if *result != "" {
		fmt.Printf("[%s]: ", *result)
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
	newIssueURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/issues", url.QueryEscape(GITLAB_RUNNER_PROJECT_ID))

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
