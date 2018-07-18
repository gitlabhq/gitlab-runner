package shells

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
)

type AbstractShell struct {
}

func (b *AbstractShell) GetFeatures(features *common.FeaturesInfo) {
	features.Artifacts = true
	features.Cache = true
	features.ArtifactsFormat = true
}

func (b *AbstractShell) writeCdBuildDir(w ShellWriter, info common.ShellScriptInfo) {
	w.Cd(info.Build.FullProjectDir())
}

func (b *AbstractShell) writeExports(w ShellWriter, info common.ShellScriptInfo) {
	for _, variable := range info.Build.GetAllVariables() {
		w.Variable(variable)
	}
}

func (b *AbstractShell) writeGitSSLConfig(w ShellWriter, build *common.Build, where []string) {
	repoURL, err := url.Parse(build.Runner.URL)
	if err != nil {
		w.Warning("git SSL config: Can't parse repository URL. %s", err)
		return
	}

	repoURL.Path = ""
	host := repoURL.String()
	variables := build.GetCITLSVariables()
	args := append([]string{"config"}, where...)

	for variable, config := range map[string]string{
		tls.VariableCAFile:   "sslCAInfo",
		tls.VariableCertFile: "sslCert",
		tls.VariableKeyFile:  "sslKey",
	} {
		if variables.Get(variable) == "" {
			continue
		}

		key := fmt.Sprintf("http.%s.%s", host, config)
		value := w.TmpFile(variable)
		w.Command("git", append(args, key, value)...)
	}

	return
}

func (b *AbstractShell) writeCloneCmd(w ShellWriter, build *common.Build, projectDir string) {
	templateDir := w.MkTmpDir("git-template")
	args := []string{"clone", "--no-checkout", build.GetRemoteURL(), projectDir, "--template", templateDir}

	w.RmDir(projectDir)
	templateFile := path.Join(templateDir, "config")
	w.Command("git", "config", "-f", templateFile, "fetch.recurseSubmodules", "false")
	if build.IsSharedEnv() {
		b.writeGitSSLConfig(w, build, []string{"-f", templateFile})
	}

	if depth := build.GetGitDepth(); depth != "" {
		w.Notice("Cloning repository for %s with git depth set to %s...", build.GitInfo.Ref, depth)
		args = append(args, "--depth", depth, "--branch", build.GitInfo.Ref)
	} else {
		w.Notice("Cloning repository...")
	}

	w.Command("git", args...)
	w.Cd(projectDir)
}

func (b *AbstractShell) writeFetchCmd(w ShellWriter, build *common.Build, projectDir string, gitDir string) {
	depth := build.GetGitDepth()

	w.IfDirectory(gitDir)
	if depth != "" {
		w.Notice("Fetching changes for %s with git depth set to %s...", build.GitInfo.Ref, depth)
	} else {
		w.Notice("Fetching changes...")
	}
	w.Cd(projectDir)
	w.Command("git", "config", "fetch.recurseSubmodules", "false")

	if build.IsSharedEnv() {
		b.writeGitSSLConfig(w, build, nil)
	}

	// Remove .git/{index,shallow,HEAD}.lock files from .git, which can fail the fetch command
	// The file can be left if previous build was terminated during git operation
	w.RmFile(".git/index.lock")
	w.RmFile(".git/shallow.lock")
	w.RmFile(".git/HEAD.lock")

	w.IfFile(".git/hooks/post-checkout")
	w.RmFile(".git/hooks/post-checkout")
	w.EndIf()

	w.Command("git", "clean", "-ffdx")
	w.Command("git", "reset", "--hard")
	w.Command("git", "remote", "set-url", "origin", build.GetRemoteURL())
	if depth != "" {
		var refspec string
		if build.GitInfo.RefType == common.RefTypeTag {
			refspec = "+refs/tags/" + build.GitInfo.Ref + ":refs/tags/" + build.GitInfo.Ref
		} else {
			refspec = "+refs/heads/" + build.GitInfo.Ref + ":refs/remotes/origin/" + build.GitInfo.Ref
		}
		w.Command("git", "fetch", "--depth", depth, "origin", "--prune", refspec)
	} else {
		w.Command("git", "fetch", "origin", "--prune", "+refs/heads/*:refs/remotes/origin/*", "+refs/tags/*:refs/tags/*")
	}
	w.Else()
	b.writeCloneCmd(w, build, projectDir)
	w.EndIf()
}

func (b *AbstractShell) writeCheckoutCmd(w ShellWriter, build *common.Build) {
	w.Notice("Checking out %s as %s...", build.GitInfo.Sha[0:8], build.GitInfo.Ref)
	w.Command("git", "checkout", "-f", "-q", build.GitInfo.Sha)
}

func (b *AbstractShell) writeSubmoduleUpdateCmd(w ShellWriter, build *common.Build, recursive bool) {
	if recursive {
		w.Notice("Updating/initializing submodules recursively...")
	} else {
		w.Notice("Updating/initializing submodules...")
	}

	// Sync .git/config to .gitmodules in case URL changes (e.g. new build token)
	args := []string{"submodule", "sync"}
	if recursive {
		args = append(args, "--recursive")
	}
	w.Command("git", args...)

	// Update / initialize submodules
	updateArgs := []string{"submodule", "update", "--init"}
	foreachArgs := []string{"submodule", "foreach"}
	if recursive {
		updateArgs = append(updateArgs, "--recursive")
		foreachArgs = append(foreachArgs, "--recursive")
	}

	// Clean changed files in submodules
	// "git submodule update --force" option not supported in Git 1.7.1 (shipped with CentOS 6)
	w.Command("git", append(foreachArgs, "git", "clean", "-ffxd")...)
	w.Command("git", append(foreachArgs, "git", "reset", "--hard")...)
	w.Command("git", updateArgs...)
}

func (b *AbstractShell) cacheFile(build *common.Build, userKey string) (key, file string) {
	if build.CacheDir == "" {
		return
	}

	// Deduce cache key
	key = path.Join(build.JobInfo.Name, build.GitInfo.Ref)
	if userKey != "" {
		key = build.GetAllVariables().ExpandValue(userKey)
	}

	// Ignore cache without the key
	if key == "" {
		return
	}

	file = path.Join(build.CacheDir, key, "cache.zip")
	file, err := filepath.Rel(build.BuildDir, file)
	if err != nil {
		return "", ""
	}
	return
}

func (b *AbstractShell) guardRunnerCommand(w ShellWriter, runnerCommand string, action string, f func()) {
	if runnerCommand == "" {
		w.Warning("%s is not supported by this executor.", action)
		return
	}

	w.IfCmd(runnerCommand, "--version")
	f()
	w.Else()
	w.Warning("Missing %s. %s is disabled.", runnerCommand, action)
	w.EndIf()
}

func (b *AbstractShell) cacheExtractor(w ShellWriter, info common.ShellScriptInfo) error {
	for _, cacheOptions := range info.Build.Cache {

		// Create list of files to extract
		archiverArgs := []string{}
		for _, path := range cacheOptions.Paths {
			archiverArgs = append(archiverArgs, "--path", path)
		}

		if cacheOptions.Untracked {
			archiverArgs = append(archiverArgs, "--untracked")
		}

		// Skip restoring cache if no cache is defined
		if len(archiverArgs) < 1 {
			continue
		}

		// Skip extraction if no cache is defined
		cacheKey, cacheFile := b.cacheFile(info.Build, cacheOptions.Key)
		if cacheKey == "" {
			w.Notice("Skipping cache extraction due to empty cache key")
			continue
		}

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPull); err != nil {
			return fmt.Errorf("%s for %s", err, cacheKey)
		} else if !ok {
			w.Notice("Not downloading cache %s due to policy", cacheKey)
			continue
		}

		args := []string{
			"cache-extractor",
			"--file", cacheFile,
			"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
		}

		// Generate cache download address
		if url := getCacheDownloadURL(info.Build, cacheKey); url != nil {
			args = append(args, "--url", url.String())
		}

		// Execute cache-extractor command. Failure is not fatal.
		b.guardRunnerCommand(w, info.RunnerCommand, "Extracting cache", func() {
			w.Notice("Checking cache for %s...", cacheKey)
			w.IfCmdWithOutput(info.RunnerCommand, args...)
			w.Notice("Successfully extracted cache")
			w.Else()
			w.Warning("Failed to extract cache")
			w.EndIf()
		})
	}

	return nil
}

func (b *AbstractShell) downloadArtifacts(w ShellWriter, job common.Dependency, info common.ShellScriptInfo) {
	args := []string{
		"artifacts-downloader",
		"--url",
		info.Build.Runner.URL,
		"--token",
		job.Token,
		"--id",
		strconv.Itoa(job.ID),
	}

	w.Notice("Downloading artifacts for %s (%d)...", job.Name, job.ID)
	w.Command(info.RunnerCommand, args...)
}

func (b *AbstractShell) jobArtifacts(info common.ShellScriptInfo) (otherJobs []common.Dependency) {
	for _, otherJob := range info.Build.Dependencies {
		if otherJob.ArtifactsFile.Filename == "" {
			continue
		}

		otherJobs = append(otherJobs, otherJob)
	}
	return
}

func (b *AbstractShell) downloadAllArtifacts(w ShellWriter, info common.ShellScriptInfo) {
	otherJobs := b.jobArtifacts(info)
	if len(otherJobs) == 0 {
		return
	}

	b.guardRunnerCommand(w, info.RunnerCommand, "Artifacts downloading", func() {
		for _, otherJob := range otherJobs {
			b.downloadArtifacts(w, otherJob, info)
		}
	})
}

func (b *AbstractShell) writePrepareScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	return nil
}

func (b *AbstractShell) writeCloneFetchCmds(w ShellWriter, info common.ShellScriptInfo) (err error) {
	build := info.Build
	projectDir := build.FullProjectDir()
	gitDir := path.Join(build.FullProjectDir(), ".git")

	switch info.Build.GetGitStrategy() {
	case common.GitFetch:
		b.writeFetchCmd(w, build, projectDir, gitDir)
	case common.GitClone:
		b.writeCloneCmd(w, build, projectDir)
	case common.GitNone:
		w.Notice("Skipping Git repository setup")
		w.MkDir(projectDir)
	default:
		return errors.New("unknown GIT_STRATEGY")
	}

	if info.Build.GetGitCheckout() {
		b.writeCheckoutCmd(w, build)
	} else {
		w.Notice("Skipping Git checkout")
	}

	return nil
}

func (b *AbstractShell) writeSubmoduleUpdateCmds(w ShellWriter, info common.ShellScriptInfo) (err error) {
	build := info.Build

	switch build.GetSubmoduleStrategy() {
	case common.SubmoduleNormal:
		b.writeSubmoduleUpdateCmd(w, build, false)

	case common.SubmoduleRecursive:
		b.writeSubmoduleUpdateCmd(w, build, true)

	case common.SubmoduleNone:
		w.Notice("Skipping Git submodules setup")

	default:
		return errors.New("unknown GIT_SUBMODULE_STRATEGY")
	}

	return nil
}

func (b *AbstractShell) writeGetSourcesScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)

	if !info.Build.IsSharedEnv() {
		b.writeGitSSLConfig(w, info.Build, []string{"--global"})
	}

	if info.PreCloneScript != "" && info.Build.GetGitStrategy() != common.GitNone {
		b.writeCommands(w, info.PreCloneScript)
	}

	if err := b.writeCloneFetchCmds(w, info); err != nil {
		return err
	}

	return b.writeSubmoduleUpdateCmds(w, info)
}

func (b *AbstractShell) writeRestoreCacheScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Try to restore from main cache, if not found cache for master
	return b.cacheExtractor(w, info)
}

func (b *AbstractShell) writeDownloadArtifactsScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Process all artifacts
	b.downloadAllArtifacts(w, info)
	return nil
}

// Write the given string of commands using the provided ShellWriter object.
func (b *AbstractShell) writeCommands(w ShellWriter, commands ...string) {
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			lines := strings.SplitN(command, "\n", 2)
			if len(lines) > 1 {
				// TODO: this should be collapsable once we introduce that in GitLab
				w.Notice("$ %s # collapsed multi-line command", lines[0])
			} else {
				w.Notice("$ %s", lines[0])
			}
		} else {
			w.EmptyLine()
		}
		w.Line(command)
		w.CheckForErrors()
	}
}

func (b *AbstractShell) writeUserScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	var scriptStep *common.Step
	for _, step := range info.Build.Steps {
		if step.Name == common.StepNameScript {
			scriptStep = &step
			break
		}
	}

	if scriptStep == nil {
		return nil
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	if info.PreBuildScript != "" {
		b.writeCommands(w, info.PreBuildScript)
	}

	b.writeCommands(w, scriptStep.Script...)

	if info.PostBuildScript != "" {
		b.writeCommands(w, info.PostBuildScript)
	}

	return nil
}

func (b *AbstractShell) cacheArchiver(w ShellWriter, info common.ShellScriptInfo) error {
	for _, cacheOptions := range info.Build.Cache {
		// Skip archiving if no cache is defined
		cacheKey, cacheFile := b.cacheFile(info.Build, cacheOptions.Key)
		if cacheKey == "" {
			w.Notice("Skipping cache archiving due to empty cache key")
			continue
		}

		if ok, err := cacheOptions.CheckPolicy(common.CachePolicyPush); err != nil {
			return fmt.Errorf("%s for %s", err, cacheKey)
		} else if !ok {
			w.Notice("Not uploading cache %s due to policy", cacheKey)
			continue
		}

		args := []string{
			"cache-archiver",
			"--file", cacheFile,
			"--timeout", strconv.Itoa(info.Build.GetCacheRequestTimeout()),
		}

		// Create list of files to archive
		archiverArgs := []string{}
		for _, path := range cacheOptions.Paths {
			archiverArgs = append(archiverArgs, "--path", path)
		}

		if cacheOptions.Untracked {
			archiverArgs = append(archiverArgs, "--untracked")
		}

		if len(archiverArgs) < 1 {
			// Skip creating archive
			continue
		}
		args = append(args, archiverArgs...)

		// Generate cache upload address
		if url := getCacheUploadURL(info.Build, cacheKey); url != nil {
			args = append(args, "--url", url.String())
		}

		// Execute cache-archiver command. Failure is not fatal.
		b.guardRunnerCommand(w, info.RunnerCommand, "Creating cache", func() {
			w.Notice("Creating cache %s...", cacheKey)
			w.IfCmdWithOutput(info.RunnerCommand, args...)
			w.Notice("Created cache")
			w.Else()
			w.Warning("Failed to create cache")
			w.EndIf()
		})
	}

	return nil
}

func (b *AbstractShell) uploadArtifacts(w ShellWriter, info common.ShellScriptInfo, onSuccess bool) {
	if info.Build.Runner.URL == "" {
		return
	}

	for _, artifacts := range info.Build.Artifacts {
		if onSuccess {
			if !artifacts.When.OnSuccess() {
				continue
			}
		} else {
			if !artifacts.When.OnFailure() {
				continue
			}
		}

		args := []string{
			"artifacts-uploader",
			"--url",
			info.Build.Runner.URL,
			"--token",
			info.Build.Token,
			"--id",
			strconv.Itoa(info.Build.ID),
			"--artifact-format",
			string(artifacts.Format),
			"--artifact-type",
			artifacts.Type,
		}

		// Create list of files to archive
		archiverArgs := []string{}
		for _, path := range artifacts.Paths {
			archiverArgs = append(archiverArgs, "--path", path)
		}

		if artifacts.Untracked {
			archiverArgs = append(archiverArgs, "--untracked")
		}

		if len(archiverArgs) < 1 {
			// Skip creating archive
			continue
		}
		args = append(args, archiverArgs...)

		if artifacts.Name != "" {
			args = append(args, "--name", artifacts.Name)
		}

		if artifacts.ExpireIn != "" {
			args = append(args, "--expire-in", artifacts.ExpireIn)
		}

		if artifacts.Format != "" {
			args = append(args, "--artifact-format", string(artifacts.Format))
		}

		if artifacts.Type != "" {
			args = append(args, "--artifact-type", artifacts.Type)
		}

		b.guardRunnerCommand(w, info.RunnerCommand, "Uploading artifacts", func() {
			w.Notice("Uploading artifacts...")
			w.Command(info.RunnerCommand, args...)
		})
	}
}

func (b *AbstractShell) writeAfterScript(w ShellWriter, info common.ShellScriptInfo) error {
	var afterScriptStep *common.Step
	for _, step := range info.Build.Steps {
		if step.Name == common.StepNameAfterScript {
			afterScriptStep = &step
			break
		}
	}

	if afterScriptStep == nil {
		return nil
	}

	if len(afterScriptStep.Script) == 0 {
		return nil
	}

	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	w.Notice("Running after script...")
	b.writeCommands(w, afterScriptStep.Script...)
	return nil
}

func (b *AbstractShell) writeArchiveCacheScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Find cached files and archive them
	return b.cacheArchiver(w, info)
}

func (b *AbstractShell) writeUploadArtifactsOnSuccessScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Upload artifacts
	b.uploadArtifacts(w, info, true)
	return
}

func (b *AbstractShell) writeUploadArtifactsOnFailureScript(w ShellWriter, info common.ShellScriptInfo) (err error) {
	b.writeExports(w, info)
	b.writeCdBuildDir(w, info)

	// Upload artifacts
	b.uploadArtifacts(w, info, false)
	return
}

func (b *AbstractShell) writeScript(w ShellWriter, buildStage common.BuildStage, info common.ShellScriptInfo) error {
	methods := map[common.BuildStage]func(ShellWriter, common.ShellScriptInfo) error{
		common.BuildStagePrepare:                  b.writePrepareScript,
		common.BuildStageGetSources:               b.writeGetSourcesScript,
		common.BuildStageRestoreCache:             b.writeRestoreCacheScript,
		common.BuildStageDownloadArtifacts:        b.writeDownloadArtifactsScript,
		common.BuildStageUserScript:               b.writeUserScript,
		common.BuildStageAfterScript:              b.writeAfterScript,
		common.BuildStageArchiveCache:             b.writeArchiveCacheScript,
		common.BuildStageUploadOnSuccessArtifacts: b.writeUploadArtifactsOnSuccessScript,
		common.BuildStageUploadOnFailureArtifacts: b.writeUploadArtifactsOnFailureScript,
	}

	fn := methods[buildStage]
	if fn == nil {
		return errors.New("Not supported script type: " + string(buildStage))
	}

	return fn(w, info)
}
