package spec

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
)

type Variables []Variable

func (b Variable) String() string {
	return fmt.Sprintf("%s=%s", b.Key, b.Value)
}

const TempProjectDirVariableKey = "RUNNER_TEMP_PROJECT_DIR"

// tmpFile will return a canonical temp file path by prepending the job
// variables Key with the value of `RUNNER_TEMP_PROJECT_DIR` (typically the
// build's temporary directory). The returned path must be further expanded
// by/for each shell that uses it.
func (b Variables) tmpFile(s string) string {
	return path.Join(b.Value(TempProjectDirVariableKey), s)
}

func (b Variables) PublicOrInternal() (variables Variables) {
	for _, variable := range b {
		if variable.Public || variable.Internal {
			variables = append(variables, variable)
		}
	}
	return variables
}

func (b Variables) StringList() (variables []string) {
	for _, variable := range b {
		// For file-type secrets, substitute the path to the secret for the secret
		// value.
		if variable.File {
			v := variable
			v.Value = b.value(v.Key, true)
			variables = append(variables, v.String())
		} else {
			variables = append(variables, variable.String())
		}
	}
	return variables
}

// GetAllVariableNames returns a semicolon-separated list of all variable names
// that are set in the build. This function is used to pass the list of job variable
// names to the build container via an environment variable (e.g., RUNNER_JOB_VAR_NAMES),
// allowing step-runner to identify and filter out job variables from the OS environment.
func (b Variables) GetAllVariableNames() string {
	names := make([]string, 0, len(b))
	for _, variable := range b {
		names = append(names, variable.Key)
	}

	return strings.Join(names, ";")
}

// Get returns the value of a variable, or if a file type variable, the
// pathname to the saved file containing the value,
func (b Variables) Get(key string) string {
	return b.value(key, true)
}

// Set sets newJobVars on the JobVariables, replacing all existing variables with the same key.
// If newJobVars holds variables with the same key, only the last one is set.
func (b *Variables) Set(newJobVars ...Variable) {
	if len(newJobVars) < 1 {
		return
	}

	newVarsByKey := make(map[string]Variable, len(newJobVars))

	for _, v := range newJobVars {
		// for multiple newJobVars with the same key, only keep the last one
		newVarsByKey[v.Key] = v
	}

	*b = slices.DeleteFunc(*b, func(v Variable) bool {
		_, exists := newVarsByKey[v.Key]
		return exists
	})

	for _, v := range newVarsByKey {
		*b = append(*b, v)
	}
}

// Value is similar to Get(), but always returns the key value, regardless
// of the variable type. File variables therefore return the file contents
// and not the path name of the file.
func (b Variables) Value(key string) string {
	return b.value(key, false)
}

// value returns the contents of the variable by key.
//
// If the variable type is 'file' and the 'pathnames' parameter is true, then
// the pathname of the file containing the contents is returned instead.
func (b Variables) value(key string, pathnames bool) string {
	switch key {
	case "$":
		return key
	case "*", "#", "@", "!", "?", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return ""
	}
	for i := len(b) - 1; i >= 0; i-- {
		if b[i].Key == key {
			if b[i].File && pathnames {
				return b.tmpFile(b[i].Key)
			}
			return b[i].Value
		}
	}
	return ""
}

// Bool tries to get the boolean value of a variable
// "true" and "false" strings are parsed as well as numeric values
// where only the value of "1" is considered to be true
func (b Variables) Bool(key string) bool {
	value := b.Get(key)
	parsedBool, err := strconv.ParseBool(strings.ToLower(value))
	if err == nil {
		return parsedBool
	}

	parsedInt, err := strconv.ParseInt(value, 10, 32)
	if err == nil {
		return parsedInt == 1
	}

	return false
}

// OverwriteKey overwrites an existing key with a new variable.
func (b Variables) OverwriteKey(key string, variable Variable) {
	for i, v := range b {
		if v.Key == key {
			b[i] = variable
			return
		}
	}
}

func (b Variables) ExpandValue(value string) string {
	return os.Expand(value, b.Get)
}

func (b Variables) Expand() Variables {
	var variables Variables
	for _, variable := range b {
		if !variable.Raw {
			variable.Value = b.ExpandValue(variable.Value)
		}

		variables = append(variables, variable)
	}
	return variables
}

func (b Variables) Masked() (masked []string) {
	for _, variable := range b {
		if variable.Masked {
			masked = append(masked, variable.Value)
		}
	}
	return
}

// Dedup returns a clone of the JobVariables, where variables with the same key get de-duplicated.
// If keepOriginal is true, the first duplicate JobVariable (ie. the original value) is kept, else the last one (ie. the
// final overridden value).
// The order of variables is not preserved.
func (b Variables) Dedup(keepOriginal bool) Variables {
	clone := slices.Clone(b)

	if !keepOriginal {
		// GitLab might give us multiple vars with the same key, with the last one being the final overridden one. In order
		// to get the original value, we thus reverse the vars, and therefore get the first/original value after doing "sort
		// | uniq".
		slices.Reverse(clone)
	}

	slices.SortStableFunc(clone, func(a, b Variable) int {
		return cmp.Compare(a.Key, b.Key)
	})

	return slices.Clip(slices.CompactFunc(clone, func(a, b Variable) bool {
		return a.Key == b.Key
	}))
}
