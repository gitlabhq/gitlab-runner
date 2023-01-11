package common

import (
	"errors"
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
)

type VerifyAllowedImageOptions struct {
	Image          string
	OptionName     string
	AllowedImages  []string
	InternalImages []string
}

var ErrDisallowedImage = errors.New("disallowed image")

func VerifyAllowedImage(options VerifyAllowedImageOptions, logger BuildLogger) error {
	for _, allowedImage := range options.AllowedImages {
		ok, _ := doublestar.Match(allowedImage, options.Image)
		if ok {
			return nil
		}
	}

	for _, internalImage := range options.InternalImages {
		if internalImage == options.Image {
			return nil
		}
	}

	if len(options.AllowedImages) != 0 {
		logger.Println()
		logger.Errorln(
			fmt.Sprintf("The %q image is not present on list of allowed %s:", options.Image, options.OptionName),
		)
		for _, allowedImage := range options.AllowedImages {
			logger.Println("-", allowedImage)
		}
		logger.Println()
	} else {
		// by default allow to override the image name
		return nil
	}

	logger.Println(
		`Please check runner's allowed_images configuration: ` +
			`https://docs.gitlab.com/runner/configuration/advanced-configuration.html`,
	)

	return ErrDisallowedImage
}
