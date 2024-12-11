package provision

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"text/template"

	"github.com/magefile/mage/sh"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/kubernetes/docs"
)

const (
	baseName  = "k8s-runner-integration-tests-runner"
	namespace = "k8s-runner-integration-tests"
)

func ProvisionIntegrationKubernetes(id string) error {
	permissions, err := docs.ParsePermissions()
	if err != nil {
		return err
	}

	name := baseName + "-" + id

	role, err := docs.GeneratePermissionsDocsRoleYaml(name, namespace, map[string]string{
		"test.k8s.gitlab.com/name": name,
	}, permissions)
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"role": []byte(role),
	}

	for _, file := range []string{"rolebinding", "serviceaccount"} {
		manifest, err := renderManifest(name, file)
		if err != nil {
			return err
		}

		files[file] = manifest
	}

	for name, yamlBytes := range files {
		cmd := exec.Command("kubectl", "apply", "-f", "-")
		cmd.Stdin = bytes.NewReader(yamlBytes)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error applying %s yaml: %w", name, err)
		}
	}

	return nil
}

func renderManifest(name, file string) ([]byte, error) {
	manifest, err := os.ReadFile("magefiles/kubernetes/provision/manifests/" + file + ".yaml.tpl")
	if err != nil {
		return nil, err
	}

	tpl := template.New(file)
	tpl, err = tpl.Parse(string(manifest))
	if err != nil {
		return nil, fmt.Errorf("error parsing %s template: %w", file, err)
	}

	buffer := new(bytes.Buffer)
	err = tpl.Execute(buffer, map[string]any{
		"Name":      name,
		"Namespace": namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("error executing %s template: %w", file, err)
	}

	return buffer.Bytes(), nil
}

func DestroyIntegrationKubernetes(id string) error {
	name := baseName + "-" + id

	for _, file := range []string{"rolebinding", "serviceaccount"} {
		manifest, err := renderManifest(name, file)
		if err != nil {
			return err
		}

		cmd := exec.Command("kubectl", "delete", "--ignore-not-found=true", "-f", "-")
		cmd.Stdin = bytes.NewReader(manifest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error deleting %s yaml: %w", name, err)
		}
	}

	if err := sh.RunV("kubectl", "delete", "--ignore-not-found=true", "role", "-n", namespace, name); err != nil {
		return fmt.Errorf("error deleting role: %w", err)
	}

	return nil
}
