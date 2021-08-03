package vm

import (
	"errors"
	"fmt"
	"regexp"

	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type Executor struct {
	executors.AbstractExecutor

	allowedImages []*regexp.Regexp
}

func (e *Executor) GetBaseName(defaultBaseName string) (string, error) {
	imageName := e.Build.GetAllVariables().ExpandValue(e.Build.Image.Name)

	// Use default name if no build image specified or name is identical to a default one.
	if imageName == "" || imageName == defaultBaseName {
		return defaultBaseName, nil
	}

	if len(e.allowedImages) == 0 {
		// Ignore YAML's image if no allowed_images parameter is provided for the sake of backward compatibility.
		// And warn user about this.
		e.Warningln(fmt.Sprintf(
			"No allowed_images configuration found for \"%s\", using image \"%s\"",
			e.Build.Image.Name,
			defaultBaseName,
		))
		return defaultBaseName, nil
	}

	for _, allowedImage := range e.allowedImages {
		if allowedImage.MatchString(imageName) {
			return imageName, nil
		}
	}

	e.Println()
	e.Errorln(fmt.Sprintf("The %q image is not present on list of allowed images", imageName))
	for _, allowedImage := range e.allowedImages {
		e.Println("-", allowedImage)
	}
	e.Println()
	e.Println("Please check runner's configuration")

	return "", errors.New("invalid image")
}

func (e *Executor) ValidateAllowedImages(allowedImages []string) error {
	for _, allowedImage := range allowedImages {
		re, err := regexp.Compile(allowedImage)
		if err != nil {
			return fmt.Errorf("invalid regexp pattern in allowed_images parameter: %s", allowedImage)
		}
		e.allowedImages = append(e.allowedImages, re)
	}
	return nil
}
