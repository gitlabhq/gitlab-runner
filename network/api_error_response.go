package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
)

type APIValidationErrorResponse map[string]map[string][]string
type APIGenericErrorResponse map[string]string

func (a APIValidationErrorResponse) ErrorMessages() []string {
	problems, ok := a["message"]
	if !ok {
		return []string{"Unknown error"}
	}

	out := []string{}
	for key, messages := range problems {
		for _, message := range messages {
			out = append(out, fmt.Sprintf("%s: %s", key, message))
		}
	}

	return out
}

func (g APIGenericErrorResponse) Message() string {
	problem, ok := g["message"]
	if !ok {
		return "Unknown error"
	}

	return problem
}

func logAPIErrorMessages(res *http.Response) (err error) {
	mimeType, err := getResponseMimeType(res)
	if err != nil {
		return err
	}

	if mimeType != "application/json" {
		return fmt.Errorf("Server should return application/json. Got: %v", mimeType)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)

	err = logAPIValidationErrorMessages(bodyBytes)
	if err != nil {
		err = logAPIGenericErrorMessages(bodyBytes)
		if err != nil {
			return fmt.Errorf("Error decoding json payload %v", err)
		}
	}

	return nil
}

func logAPIValidationErrorMessages(bodyBytes []byte) (err error) {
	var validationErrorResponse APIValidationErrorResponse
	err = json.Unmarshal(bodyBytes, &validationErrorResponse)
	if err != nil {
		return err
	}

	for _, message := range validationErrorResponse.ErrorMessages() {
		logrus.Errorln(message)
	}

	return nil
}

func logAPIGenericErrorMessages(bodyBytes []byte) (err error) {
	var genericErrorResponse APIGenericErrorResponse
	err = json.Unmarshal(bodyBytes, &genericErrorResponse)
	if err != nil {
		return err
	}

	message := genericErrorResponse.Message()
	logrus.Errorln(message)

	return nil
}
