package gitlab

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

const (
	defaultBaseURL = "https://gitlab.com/"
)

type ClientNotCreatedError struct {
	inner error
}

func NewClientNotCreatedError(err error) *ClientNotCreatedError {
	return &ClientNotCreatedError{inner: err}
}

func (e *ClientNotCreatedError) Error() string {
	return fmt.Sprintf("couldn't create base GitLab client: %v", e.inner)
}

func (e *ClientNotCreatedError) Unwrap() error {
	return e.inner
}

func (e *ClientNotCreatedError) Is(err error) bool {
	_, ok := err.(*ClientNotCreatedError)

	return ok
}

type APIError struct {
	inner        error
	endpointType string
}

func NewAPIError(endpointType string, err error) *APIError {
	return &APIError{
		endpointType: endpointType,
		inner:        err,
	}
}

func (e *APIError) Error() string {
	return fmt.Sprintf("error while requesting %s from API: %v", e.endpointType, e.inner)
}

func (e *APIError) Unwrap() error {
	return e.inner
}

func (e *APIError) Is(err error) bool {
	_, ok := err.(*APIError)

	return ok
}

type MergeRequest struct {
	IID    int
	Title  string
	Labels []string

	AuthorName   string
	AuthorHandle string
}

func (mr MergeRequest) Author() string {
	if mr.AuthorName == "" {
		return ""
	}

	if mr.AuthorHandle == "" {
		return mr.AuthorName
	}

	return fmt.Sprintf("%s @%s", mr.AuthorName, mr.AuthorHandle)
}

type Client interface {
	ListMergeRequests(IIDs []int, perPage int) ([]MergeRequest, error)
}

func NewClient(privateToken string, projectID string) (Client, error) {
	return NewClientWithBaseURL(privateToken, projectID, defaultBaseURL)
}

func NewClientWithBaseURL(privateToken string, projectID string, baseURL string) (Client, error) {
	glClient := gitlab.NewClient(nil, privateToken)
	err := glClient.SetBaseURL(baseURL)
	if err != nil {
		return nil, &ClientNotCreatedError{inner: err}
	}

	adapter := &gitlabAdapter{
		projectID:  projectID,
		baseClient: glClient,
	}

	return adapter, nil
}

type gitlabAdapter struct {
	baseClient *gitlab.Client

	projectID string
}

func (a *gitlabAdapter) ListMergeRequests(IIDs []int, perPage int) ([]MergeRequest, error) {
	mrs := make([]MergeRequest, 0)

	listMROpts := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    1,
		},
		IIDs: IIDs,
	}

	for {
		logrus.WithField("page", listMROpts.Page).
			Debug("Requesting MR details from GitLab")

		mergeRequests, response, err := a.baseClient.MergeRequests.ListProjectMergeRequests(a.projectID, listMROpts)
		if err != nil {
			return nil, NewAPIError("merge requests", err)
		}

		for _, mr := range mergeRequests {
			newMR := MergeRequest{
				IID:    mr.IID,
				Title:  mr.Title,
				Labels: mr.Labels,
			}

			if mr.Author != nil {
				newMR.AuthorName = mr.Author.Name
				newMR.AuthorHandle = mr.Author.Username
			}

			mrs = append(mrs, newMR)
		}

		if response.CurrentPage >= response.TotalPages {
			break
		}

		listMROpts.Page = response.NextPage
	}

	return mrs, nil
}
