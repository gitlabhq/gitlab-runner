package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
)

type APIErrorResponse map[string]map[string][]string

func logAPIErrorMessages(res *http.Response) (err error) {
	mimeType, err := getResponseMimeType(res)
	if err != nil {
		return err
	}

	if mimeType != "application/json" {
		if mimeType == "text/plain" {
			logrus.Errorln(ioutil.ReadAll(res.Body))
			return nil
		}

		return fmt.Errorf("Server should return application/json. Got: %v", mimeType)
	}

	var apiErrorResponse APIErrorResponse
	err = json.NewDecoder(res.Body).Decode(&apiErrorResponse)
	if err != nil {
		return fmt.Errorf("Error decoding json payload %v", err)
	}

	for _, message := range apiErrorResponse.ErrorMessages() {
		logrus.Errorln(message)
	}

	return nil
}

func (a APIErrorResponse) ErrorMessages() []string {
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
