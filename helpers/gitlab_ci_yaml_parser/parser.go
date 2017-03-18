package gitlab_ci_yaml_parser

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"

	"gopkg.in/yaml.v2"
)

type GitLabCiYamlParser struct {
	jobName   string
	config    DataBag
	jobConfig DataBag
}

func (c *GitLabCiYamlParser) parseFile() (err error) {
	data, err := ioutil.ReadFile(".gitlab-ci.yml")
	if err != nil {
		return err
	}

	config := make(DataBag)
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return err
	}

	err = config.Sanitize()
	if err != nil {
		return err
	}

	c.config = config

	return
}

func (c *GitLabCiYamlParser) loadJob() (err error) {
	jobConfig, ok := c.config.GetSubOptions(c.jobName)
	if !ok {
		return fmt.Errorf("no job named %q", c.jobName)
	}

	c.jobConfig = jobConfig

	return
}

func (c *GitLabCiYamlParser) prepareJobInfo(job *common.JobResponse) (err error) {
	job.JobInfo = common.JobInfo{
		Name: c.jobName,
	}

	if stage, ok := c.jobConfig.GetString("stage"); ok {
		job.JobInfo.Stage = stage
	} else {
		job.JobInfo.Stage = "test"
	}

	return
}

func (c *GitLabCiYamlParser) getCommands(commands interface{}) (common.StepScript, error) {
	if lines, ok := commands.([]interface{}); ok {
		text := ""
		for _, line := range lines {
			if lineText, ok := line.(string); ok {
				text += lineText + "\n"
			} else {
				return common.StepScript{}, errors.New("unsupported script")
			}
		}
		return common.StepScript(strings.Split(text, "\n")), nil
	} else if text, ok := commands.(string); ok {
		return common.StepScript(strings.Split(text, "\n")), nil
	} else if commands != nil {
		return common.StepScript{}, errors.New("unsupported script")
	}

	return common.StepScript{}, nil
}

func (c *GitLabCiYamlParser) prepareSteps(job *common.JobResponse) (err error) {
	if c.jobConfig["script"] == nil {
		err = fmt.Errorf("missing 'script' for job")
		return
	}

	var scriptCommands, afterScriptCommands common.StepScript

	// get before_script
	beforeScript, err := c.getCommands(c.config["before_script"])
	if err != nil {
		return
	}

	// get job before_script
	jobBeforeScript, err := c.getCommands(c.jobConfig["before_script"])
	if err != nil {
		return
	}

	if len(jobBeforeScript) < 1 {
		scriptCommands = beforeScript
	} else {
		scriptCommands = jobBeforeScript
	}

	// get script
	script, err := c.getCommands(c.jobConfig["script"])
	if err != nil {
		return
	}
	for _, scriptLine := range script {
		scriptCommands = append(scriptCommands, scriptLine)
	}

	afterScriptCommands, err = c.getCommands(c.jobConfig["after_script"])
	if err != nil {
		return
	}

	job.Steps = common.Steps{
		common.Step{
			Name:         common.StepNameScript,
			Script:       scriptCommands,
			Timeout:      3600,
			When:         common.StepWhenOnSuccess,
			AllowFailure: false,
		},
		common.Step{
			Name:         common.StepNameAfterScript,
			Script:       afterScriptCommands,
			Timeout:      3600,
			When:         common.StepWhenAlways,
			AllowFailure: false,
		},
	}

	return
}

func (c *GitLabCiYamlParser) buildVariables(configVariables interface{}) (buildVariables common.BuildVariables, err error) {
	if variables, ok := configVariables.(map[string]interface{}); ok {
		for key, value := range variables {
			if valueText, ok := value.(string); ok {
				buildVariables = append(buildVariables, common.BuildVariable{
					Key:    key,
					Value:  valueText,
					Public: true,
				})
			} else {
				err = fmt.Errorf("invalid value for variable %q", key)
			}
		}
	} else if configVariables != nil {
		err = errors.New("unsupported variables")
	}

	return
}

func (c *GitLabCiYamlParser) prepareVariables(job *common.JobResponse) (err error) {
	job.Variables = common.BuildVariables{}
	job.Variables, err = c.buildVariables(c.config["variables"])
	if err != nil {
		return
	}

	jobVariables, err := c.buildVariables(c.jobConfig["variables"])
	if err != nil {
		return
	}

	job.Variables = append(job.Variables, jobVariables...)

	return
}

func (c *GitLabCiYamlParser) prepareImage(job *common.JobResponse) (err error) {
	job.Image = common.Image{}
	if imageName, ok := getOption("image", c.config, c.jobConfig); ok {
		job.Image.Name = imageName.(string)
	}

	return
}

func (c *GitLabCiYamlParser) prepareServices(job *common.JobResponse) (err error) {
	job.Services = common.Services{}

	if servicesMap, ok := getOptions("services", c.config, c.jobConfig); ok {
		for _, service := range servicesMap {
			job.Services = append(job.Services, common.Image{
				Name: service.(string),
			})
		}
	}

	return
}

func (c *GitLabCiYamlParser) prepareArtifacts(job *common.JobResponse) (err error) {
	var ok bool

	artifactsMap := getOptionsMap("artifacts", c.config, c.jobConfig)

	artifactsPaths, _ := artifactsMap.GetSlice("paths")
	paths := common.ArtifactPaths{}
	for _, path := range artifactsPaths {
		paths = append(paths, path.(string))
	}

	var artifactsName string
	if artifactsName, ok = artifactsMap.GetString("name"); !ok {
		artifactsName = ""
	}

	var artifactsUntracked interface{}
	if artifactsUntracked, ok = artifactsMap.Get("untracked"); !ok {
		artifactsUntracked = false
	}

	var artifactsWhen string
	if artifactsWhen, ok = artifactsMap.GetString("when"); !ok {
		artifactsWhen = common.ArtifactWhenOnSuccess
	}

	var artifactsExpireIn string
	if artifactsExpireIn, ok = artifactsMap.GetString("expireIn"); !ok {
		artifactsExpireIn = ""
	}

	job.Artifacts = make(common.Artifacts, 1)
	job.Artifacts[0] = common.Artifact{
		Name:      artifactsName,
		Untracked: artifactsUntracked.(bool),
		Paths:     paths,
		When:      common.ArtifactWhen(artifactsWhen),
		ExpireIn:  artifactsExpireIn,
	}

	return
}

func (c *GitLabCiYamlParser) prepareCache(job *common.JobResponse) (err error) {
	var ok bool

	cacheMap := getOptionsMap("cache", c.config, c.jobConfig)

	cachePaths, _ := cacheMap.GetSlice("paths")
	paths := common.ArtifactPaths{}
	for _, path := range cachePaths {
		paths = append(paths, path.(string))
	}

	var cacheKey string
	if cacheKey, ok = cacheMap.GetString("key"); !ok {
		cacheKey = ""
	}

	var cacheUntracked interface{}
	if cacheUntracked, ok = cacheMap.Get("untracked"); !ok {
		cacheUntracked = false
	}

	job.Cache = make(common.Caches, 1)
	job.Cache[0] = common.Cache{
		Key:       cacheKey,
		Untracked: cacheUntracked.(bool),
		Paths:     paths,
	}

	return
}

func (c *GitLabCiYamlParser) ParseYaml(job *common.JobResponse) (err error) {
	err = c.parseFile()
	if err != nil {
		return err
	}

	err = c.loadJob()
	if err != nil {
		return err
	}

	parsers := []struct {
		method func(job *common.JobResponse) error
	}{
		{c.prepareJobInfo},
		{c.prepareSteps},
		{c.prepareVariables},
		{c.prepareImage},
		{c.prepareServices},
		{c.prepareArtifacts},
		{c.prepareCache},
	}

	for _, parser := range parsers {
		err = parser.method(job)
		if err != nil {
			return err
		}
	}

	return nil
}

func NewGitLabCiYamlParser(jobName string) *GitLabCiYamlParser {
	return &GitLabCiYamlParser{
		jobName: jobName,
	}
}
