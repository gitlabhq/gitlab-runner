package common

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

type GitStrategy string

const (
	GitClone GitStrategy = "clone"
	GitFetch GitStrategy = "fetch"
	GitNone  GitStrategy = "none"
	GitEmpty GitStrategy = "empty"
)

type cmdFlags []string

var (
	gitCleanFlagsDefault = cmdFlags{"-ffdx"}
	gitFetchFlagsDefault = cmdFlags{"--prune", "--quiet"}
)

type SubmoduleStrategy string

const (
	SubmoduleInvalid   SubmoduleStrategy = "invalid"
	SubmoduleNone      SubmoduleStrategy = "none"
	SubmoduleNormal    SubmoduleStrategy = "normal"
	SubmoduleRecursive SubmoduleStrategy = "recursive"

	DefaultObjectFormat = "sha1"
)

type BuildSettings struct {
	CIDebugServices bool
	CIDebugTrace    bool

	GitClonePath            string
	GitCheckout             bool
	GitSubmoduleStrategy    SubmoduleStrategy
	GitStrategy             GitStrategy
	GitSubmodulePaths       []string
	GitSubmoduleDepth       int
	GitCleanFlags           cmdFlags
	GitCloneExtraFlags      cmdFlags
	GitFetchExtraFlags      cmdFlags
	GitSubmoduleUpdateFlags cmdFlags
	GitLFSSkipSmudge        bool
	GitSubmoduleForceHTTPS  bool

	GetSourcesAttempts         int
	ArtifactDownloadAttempts   int
	RestoreCacheAttempts       int
	ExecutorJobSectionAttempts int

	AfterScriptIgnoreErrors bool

	CacheRequestTimeout int

	DockerAuthConfig string

	FeatureFlags map[string]bool

	Errors []error
}

// Settings returns user provided build settings.
func (b *Build) Settings() BuildSettings {
	b.initSettings()

	return *b.buildSettings
}

func (b *Build) initSettings() {
	if b.buildSettings != nil {
		return
	}

	b.buildSettings = &BuildSettings{}

	// PHASE 1: Use explicit method for feature flag resolution
	variablesForResolution := b.getVariablesForFeatureFlagResolution()

	defaultGitStrategy := GitClone
	if b.AllowGitFetch {
		defaultGitStrategy = GitFetch
	}

	errs := validateVariables(variablesForResolution, b, defaultGitStrategy)

	if b.Runner != nil && b.Runner.DebugTraceDisabled {
		if b.buildSettings.CIDebugTrace {
			errs = append(errs, fmt.Errorf("CI_DEBUG_TRACE: usage is disabled on this Runner"))
		}
		if b.buildSettings.CIDebugServices {
			errs = append(errs, fmt.Errorf("CI_DEBUG_SERVICES: usage is disabled on this Runner"))
		}
		b.buildSettings.CIDebugTrace = false
		b.buildSettings.CIDebugServices = false
	}

	if b.buildSettings.ExecutorJobSectionAttempts < 1 || b.buildSettings.ExecutorJobSectionAttempts > 10 {
		errs = append(errs, fmt.Errorf("EXECUTOR_JOB_SECTION_ATTEMPTS: number of attempts out of the range [1, 10], using default %v", DefaultExecutorStageAttempts))
		b.buildSettings.ExecutorJobSectionAttempts = DefaultExecutorStageAttempts
	}

	errs = append(errs, populateFeatureFlags(b, variablesForResolution)...)

	b.buildSettings.Errors = slices.DeleteFunc(errs, func(err error) bool {
		return err == nil
	})
}

func validateVariables(variables spec.Variables, b *Build, defaultGitStategy GitStrategy) []error {
	return []error{
		validate(variables, "CI_DEBUG_SERVICES", &b.buildSettings.CIDebugServices, false),
		validate(variables, "CI_DEBUG_TRACE", &b.buildSettings.CIDebugTrace, false),

		validate(variables, "GIT_CLONE_PATH", &b.buildSettings.GitClonePath, ""),
		validate(variables, "GIT_STRATEGY", &b.buildSettings.GitStrategy, defaultGitStategy),
		validate(variables, "GIT_CHECKOUT", &b.buildSettings.GitCheckout, true),
		validate(variables, "GIT_SUBMODULE_STRATEGY", &b.buildSettings.GitSubmoduleStrategy, SubmoduleInvalid),
		validate(variables, "GIT_SUBMODULE_PATHS", &b.buildSettings.GitSubmodulePaths, nil),
		validate(variables, "GIT_SUBMODULE_DEPTH", &b.buildSettings.GitSubmoduleDepth, b.GitInfo.Depth),
		validate(variables, "GIT_CLEAN_FLAGS", &b.buildSettings.GitCleanFlags, gitCleanFlagsDefault),
		validate(variables, "GIT_CLONE_EXTRA_FLAGS", &b.buildSettings.GitCloneExtraFlags, cmdFlags{}),
		validate(variables, "GIT_FETCH_EXTRA_FLAGS", &b.buildSettings.GitFetchExtraFlags, gitFetchFlagsDefault),
		validate(variables, "GIT_SUBMODULE_UPDATE_FLAGS", &b.buildSettings.GitSubmoduleUpdateFlags, nil),
		validate(variables, "GIT_LFS_SKIP_SMUDGE", &b.buildSettings.GitLFSSkipSmudge, false),
		validate(variables, "GIT_SUBMODULE_FORCE_HTTPS", &b.buildSettings.GitSubmoduleForceHTTPS, false),

		validate(variables, "GET_SOURCES_ATTEMPTS", &b.buildSettings.GetSourcesAttempts, DefaultGetSourcesAttempts),
		validate(variables, "ARTIFACT_DOWNLOAD_ATTEMPTS", &b.buildSettings.ArtifactDownloadAttempts, DefaultArtifactDownloadAttempts),
		validate(variables, "RESTORE_CACHE_ATTEMPTS", &b.buildSettings.RestoreCacheAttempts, DefaultRestoreCacheAttempts),
		validate(variables, "EXECUTOR_JOB_SECTION_ATTEMPTS", &b.buildSettings.ExecutorJobSectionAttempts, DefaultExecutorStageAttempts),

		validate(variables, "AFTER_SCRIPT_IGNORE_ERRORS", &b.buildSettings.AfterScriptIgnoreErrors, DefaultAfterScriptIgnoreErrors),

		validate(variables, "CACHE_REQUEST_TIMEOUT", &b.buildSettings.CacheRequestTimeout, DefaultCacheRequestTimeout),

		validate(variables, "DOCKER_AUTH_CONFIG", &b.buildSettings.DockerAuthConfig, ""),
	}
}

func validate[T any](variables spec.Variables, name string, value *T, def T) error {
	raw := variables.Value(name)
	var err error

	switch v := any(value).(type) {
	case *SubmoduleStrategy:
		switch strategy := SubmoduleStrategy(raw); strategy {
		case SubmoduleNormal, SubmoduleRecursive, SubmoduleNone:
			*v = strategy
		case "":
			*v = SubmoduleNone
		default:
			*value = def
			return fmt.Errorf("%s: expected either 'normal', 'recursive' or 'none' got %q", name, raw)
		}
		return nil

	case *GitStrategy:
		switch strategy := GitStrategy(raw); strategy {
		case GitClone, GitFetch, GitNone, GitEmpty:
			*v = strategy
		case "":
			*value = def
		default:
			*value = def
			return fmt.Errorf("%s: expected either 'clone', 'fetch', 'none' or 'empty' got %q, using default value '%v'", name, raw, def)
		}
		return nil
	}

	// all cases below use a default when the value is empty
	if raw == "" {
		*value = def
		return nil
	}

	switch v := any(value).(type) {
	case *bool:
		*v, err = strconv.ParseBool(raw)
		if err != nil {
			*value = def
			return fmt.Errorf("%s: expected bool got %q, using default value: %v", name, raw, def)
		}

	case *int:
		i, err := strconv.ParseInt(raw, 10, 64)
		*v = int(i)
		if err != nil {
			*value = def
			return fmt.Errorf("%s: expected int got %q, using default value: %v", name, raw, def)
		}

	case *string:
		*v = raw

	case *cmdFlags:
		switch raw {
		case "none":
			*v = cmdFlags{}
		default:
			*v = cmdFlags(strings.Fields(raw))
		}

	case *[]string:
		*v = strings.Fields(raw)
	}

	return nil
}

//nolint:gocognit
func populateFeatureFlags(b *Build, variables spec.Variables) []error {
	var errs []error

	// test mode only: in tests, we provide a mechanism for providing
	// feature flags via RUNNER_TEST_FEATURE_FLAGS, if the flag is present,
	// we treat it as a toggle to the default flag value.
	var testFlags []string
	if flag.Lookup("test.v") != nil {
		testFlags = strings.FieldsFunc(os.Getenv("RUNNER_TEST_FEATURE_FLAGS"), func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		})
	}

	b.buildSettings.FeatureFlags = make(map[string]bool)
	for _, ff := range featureflags.GetAll() {
		b.buildSettings.FeatureFlags[ff.Name] = ff.DefaultValue

		if len(testFlags) > 0 {
			if slices.Contains(testFlags, ff.Name) {
				b.buildSettings.FeatureFlags[ff.Name] = !ff.DefaultValue
				continue
			}
		}

		// runner setting takes precedence if defined
		if b.Runner != nil && b.Runner.FeatureFlags != nil {
			val, ok := b.Runner.FeatureFlags[ff.Name]
			if ok {
				b.buildSettings.FeatureFlags[ff.Name] = val
				continue
			}
		}

		// if job variable is valid it can override default
		raw := variables.Get(ff.Name)
		val, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%v: could not parse feature flag, expected bool, got %v", ff.Name, raw))
		} else {
			b.buildSettings.FeatureFlags[ff.Name] = val
		}
	}

	return errs
}
