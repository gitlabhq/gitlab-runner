package kubernetes

import (
	"fmt"
	"regexp"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	NamespaceOverwriteVariableName      = "KUBERNETES_NAMESPACE_OVERWRITE"
	ServiceAccountOverwriteVariableName = "KUBERNETES_SERVICE_ACCOUNT_OVERWRITE"
)

type overwrites struct {
	namespace      string
	serviceAccount string
}

func createOverwrites(config *common.KubernetesConfig, variables common.JobVariables, logger common.BuildLogger) (*overwrites, error) {
	o := &overwrites{namespace: config.Namespace, serviceAccount: config.ServiceAccount}

	if err := o.overwriteNamespace(config.NamespaceOverwriteAllowed, variables, logger); err != nil {
		return nil, err
	}

	if err := o.overwriteServiceAccount(config.ServiceAccountOverwriteAllowed, variables, logger); err != nil {
		return nil, err
	}

	return o, nil
}

// overwriteNamespace checks for variable in order to overwrite the configured
// namespace, as long as it complies to validation regular-expression, when
// expression is empty the overwrite is disabled.
func (o *overwrites) overwriteNamespace(regex string, variables common.JobVariables, logger common.BuildLogger) error {
	if regex == "" {
		logger.Debugln("Configuration entry 'namespace_overwrite_allowed' is empty, using configured namespace.")
		return nil
	}

	// looking for namespace overwrite variable, and expanding for interpolation
	namespaceOverwrite := variables.Expand().Get(NamespaceOverwriteVariableName)
	if namespaceOverwrite == "" {
		return nil
	}

	if err := overwriteRegexCheck(regex, namespaceOverwrite); err != nil {
		return err
	}

	logger.Println("Overwritting configured namespace, from", o.namespace, "to", namespaceOverwrite)
	o.namespace = namespaceOverwrite

	return nil
}

// overwriteSercviceAccount checks for variable in order to overwrite the configured
// service account, as long as it complies to validation regular-expression, when
// expression is empty the overwrite is disabled.
func (o *overwrites) overwriteServiceAccount(regex string, variables common.JobVariables, logger common.BuildLogger) error {
	if regex == "" {
		logger.Debugln("Configuration entry 'service_account_overwrite_allowed' is empty, disabling override.")
		return nil
	}

	serviceAccountOverwrite := variables.Expand().Get(ServiceAccountOverwriteVariableName)
	if serviceAccountOverwrite == "" {
		return nil
	}

	if err := overwriteRegexCheck(regex, serviceAccountOverwrite); err != nil {
		return err
	}

	logger.Println("Overwritting configured ServiceAccount, from", o.serviceAccount, "to", serviceAccountOverwrite)
	o.serviceAccount = serviceAccountOverwrite

	return nil
}

//overwriteRegexCheck check if the regex provided for overwriting a config field matches the
//paramether provided, returns error if doesn't match
func overwriteRegexCheck(regex, value string) error {
	var err error
	var r *regexp.Regexp
	if r, err = regexp.Compile(regex); err != nil {
		return err
	}

	if match := r.MatchString(value); !match {
		return fmt.Errorf("Provided value %s does not match regex %s", value, regex)
	}
	return nil
}
