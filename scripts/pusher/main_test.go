package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCleanTagFragments(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "all valid pass through",
			input: []string{"latest", "bleeding"},
			want:  []string{"latest", "bleeding"},
		},
		{
			name:  "empty strings filtered",
			input: []string{"latest", "", "bleeding"},
			want:  []string{"latest", "bleeding"},
		},
		{
			name:  "whitespace trimmed",
			input: []string{"  latest  ", "bleeding"},
			want:  []string{"latest", "bleeding"},
		},
		{
			name:  "all empty or whitespace",
			input: []string{"", "  "},
			want:  nil,
		},
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanTagFragments(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("cleanTagFragments() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildImageTags(t *testing.T) {
	tests := []struct {
		name         string
		manifest     Manifest
		repo         string
		tagFragments []string
		wantTags     map[string][]string
		wantExports  []Export
	}{
		{
			name: "single component single template single fragment",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding"},
			wantTags: map[string][]string{
				"ubuntu-x86_64": {"ubuntu-x86_64-bleeding"},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-bleeding"},
			},
		},
		{
			name: "single component multiple templates",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%", "x86_64-%"},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding"},
			wantTags: map[string][]string{
				"ubuntu-x86_64": {"ubuntu-x86_64-bleeding", "x86_64-bleeding"},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:x86_64-bleeding"},
			},
		},
		{
			name: "multiple fragments accumulate tags",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding", "latest"},
			wantTags: map[string][]string{
				"ubuntu-x86_64": {"ubuntu-x86_64-bleeding", "ubuntu-x86_64-latest"},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-latest"},
			},
		},
		{
			name: "multiple components multiple fragments",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					"ubuntu-arm64":  {"ubuntu-arm64-%"},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding", "latest"},
			wantTags: map[string][]string{
				"ubuntu-x86_64": {"ubuntu-x86_64-bleeding", "ubuntu-x86_64-latest"},
				"ubuntu-arm64":  {"ubuntu-arm64-bleeding", "ubuntu-arm64-latest"},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-latest"},
				{Type: "Docker image", Value: "example.com/repo:ubuntu-arm64-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:ubuntu-arm64-latest"},
			},
		},
		{
			name: "match map overrides default for specific fragment",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					"ubuntu-arm64":  {"ubuntu-arm64-%"},
				},
				Match: map[string]map[string][]string{
					"special": {
						"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"special"},
			wantTags: map[string][]string{
				"ubuntu-x86_64": {"ubuntu-x86_64-special"},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-x86_64-special"},
			},
		},
	}

	sortExports := cmpopts.SortSlices(func(a, b Export) bool {
		return a.Value < b.Value
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags, gotExports := tt.manifest.buildImageTags(tt.repo, tt.tagFragments)
			if diff := cmp.Diff(tt.wantTags, gotTags); diff != "" {
				t.Errorf("buildImageTags() imageTags mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantExports, gotExports, sortExports); diff != "" {
				t.Errorf("buildImageTags() exports mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Test the behavior of a populated Indexes configuration, not the generation of that configuration.
// Tests for the auto-generation of index configuration from the manifest live in indexes_test.go.
func TestBuildIndexTags(t *testing.T) {
	tests := []struct {
		name         string
		manifest     Manifest
		repo         string
		tagFragments []string
		wantIndexes  []ImageIndex
		wantExports  []Export
	}{
		{
			name: "single index single template single fragment",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					"ubuntu-arm64":  {"ubuntu-arm64-%"},
				},
				Indexes: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding"},
			wantIndexes: []ImageIndex{
				{Tags: []string{"ubuntu-bleeding"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-bleeding"},
			},
		},
		{
			name: "multiple fragments expand all templates",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					"ubuntu-arm64":  {"ubuntu-arm64-%"},
				},
				Indexes: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding", "latest"},
			wantIndexes: []ImageIndex{
				{Tags: []string{"ubuntu-bleeding", "ubuntu-latest"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:ubuntu-latest"},
			},
		},
		{
			name: "multiple index definitions with multiple fragments",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					"alpine-x86_64": {"alpine-x86_64-%"},
				},
				Indexes: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64"}},
					{Tags: []string{"alpine-%"}, Components: []string{"alpine-x86_64"}},
				},
			},
			repo:         "example.com/repo",
			tagFragments: []string{"bleeding", "latest"},
			wantIndexes: []ImageIndex{
				{Tags: []string{"ubuntu-bleeding", "ubuntu-latest"}, Components: []string{"ubuntu-x86_64"}},
				{Tags: []string{"alpine-bleeding", "alpine-latest"}, Components: []string{"alpine-x86_64"}},
			},
			wantExports: []Export{
				{Type: "Docker image", Value: "example.com/repo:ubuntu-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:ubuntu-latest"},
				{Type: "Docker image", Value: "example.com/repo:alpine-bleeding"},
				{Type: "Docker image", Value: "example.com/repo:alpine-latest"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndexes, gotExports := tt.manifest.buildIndexTags(tt.repo, tt.tagFragments)
			if diff := cmp.Diff(tt.wantIndexes, gotIndexes); diff != "" {
				t.Errorf("buildIndexTags() indexes mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantExports, gotExports); diff != "" {
				t.Errorf("buildIndexTags() exports mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name            string
		config          RuntimeConfig
		wantErr         bool
		wantErrContains []string
	}{
		{
			name: "pass: all index components present in default",
			config: RuntimeConfig{
				tagFragments: []string{"bleeding"},
				manifest: Manifest{
					Default: map[string][]string{
						"ubuntu-x86_64": {"ubuntu-x86_64-%"},
						"ubuntu-arm64":  {"ubuntu-arm64-%"},
					},
				},
				indexTags: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
				},
			},
			wantErr: false,
		},
		{
			name: "fail: index references component not in default",
			config: RuntimeConfig{
				tagFragments: []string{"bleeding"},
				manifest: Manifest{
					Default: map[string][]string{
						"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					},
				},
				indexTags: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
				},
			},
			wantErr:         true,
			wantErrContains: []string{"ubuntu-arm64", "bleeding"},
		},
		{
			name: "fail: multiple missing components produce multiple errors",
			config: RuntimeConfig{
				tagFragments: []string{"bleeding"},
				manifest: Manifest{
					Default: map[string][]string{},
				},
				indexTags: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
				},
			},
			wantErr:         true,
			wantErrContains: []string{"ubuntu-x86_64", "ubuntu-arm64"},
		},
		{
			name: "fail: match entry for fragment excludes component present in default",
			config: RuntimeConfig{
				tagFragments: []string{"special"},
				manifest: Manifest{
					Default: map[string][]string{
						"ubuntu-x86_64": {"ubuntu-x86_64-%"},
						"ubuntu-arm64":  {"ubuntu-arm64-%"},
					},
					Match: map[string]map[string][]string{
						"special": {
							"ubuntu-arm64": {"ubuntu-arm64-%"},
						},
					},
				},
				indexTags: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64", "ubuntu-arm64"}},
				},
			},
			wantErr:         true,
			wantErrContains: []string{"ubuntu-x86_64"},
		},
		{
			name: "pass: no indexes defined",
			config: RuntimeConfig{
				tagFragments: []string{"bleeding"},
				manifest: Manifest{
					Default: map[string][]string{},
				},
				indexTags: []ImageIndex{},
			},
			wantErr: false,
		},
		{
			name: "pass: no tag fragments",
			config: RuntimeConfig{
				tagFragments: []string{},
				manifest: Manifest{
					Default: map[string][]string{
						"ubuntu-x86_64": {"ubuntu-x86_64-%"},
					},
				},
				indexTags: []ImageIndex{
					{Tags: []string{"ubuntu-%"}, Components: []string{"ubuntu-x86_64"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, s := range tt.wantErrContains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("validate() error = %q, want it to contain %q", err.Error(), s)
				}
			}
		})
	}
}
