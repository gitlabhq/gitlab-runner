package kubernetes

import (
	"fmt"
	"regexp"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	// NamespaceOverwriteVariableName is the key for the JobVariable containing user overwritten Namespace
	NamespaceOverwriteVariableName = "KUBERNETES_NAMESPACE_OVERWRITE"
	// ServiceAccountOverwriteVariableName is the key for the JobVariable containing user overwritten ServiceAccount
	ServiceAccountOverwriteVariableName = "KUBERNETES_SERVICE_ACCOUNT_OVERWRITE"
)

type overwrites struct {
	namespace      string
	serviceAccount string
}

func createOverwrites(config *common.KubernetesConfig, variables common.JobVariables, logger common.BuildLogger) (*overwrites, error) {
	var err error
	o := &overwrites{}

	namespaceRegex := variables.Expand().Get(NamespaceOverwriteVariableName)
	o.namespace, err = o.evaluateOverwrite("Namespace", config.Namespace, config.NamespaceOverwriteAllowed, namespaceRegex, logger)
	if err != nil {
		return nil, err
	}

	serviceAccountRegex := variables.Expand().Get(ServiceAccountOverwriteVariableName)
	o.serviceAccount, err = o.evaluateOverwrite("ServiceAccount", config.ServiceAccount, config.ServiceAccountOverwriteAllowed, serviceAccountRegex, logger)
	if err != nil {
		return nil, err
	}

	return o, nil
}

func (o *overwrites) evaluateOverwrite(fieldName, value, regex, overwriteValue string, logger common.BuildLogger) (string, error) {
	if regex == "" {
		logger.Debugln("Regex allowing overrides for", fieldName, "is empty, disabling override.")
		return value, nil
	}

	if overwriteValue == "" {
		return value, nil
	}

	if err := overwriteRegexCheck(regex, overwriteValue); err != nil {
		return value, err
	}

	logger.Println(fmt.Sprintf("Overvriting configured %s, from %q to %q", fieldName, value, overwriteValue))

	return overwriteValue, nil
}

func overwriteRegexCheck(regex, value string) error {
	var err error
	var r *regexp.Regexp
	if r, err = regexp.Compile(regex); err != nil {
		return err
	}

	if match := r.MatchString(value); !match {
		return fmt.Errorf("Provided value %q does not match regex %q", value, regex)
	}
	return nil
}
