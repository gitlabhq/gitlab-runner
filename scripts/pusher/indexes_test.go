package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCollectIndexes(t *testing.T) {
	tests := []struct {
		name       string
		manifest   Manifest
		wantGroups []ImageIndex
		wantErr    bool
	}{
		{
			// Verifies simple grouping, with special handling for bare "%" tag,
			// which is separated out in order to include the proper windows
			// images if available.
			name: "alpine3.21 with multiple architectures",
			manifest: Manifest{
				Default: map[string][]string{
					"alpine3.21-arm64":   {"alpine3.21-arm64-%", "arm64-%"},
					"alpine3.21-arm":     {"alpine3.21-arm-%", "arm-%"},
					"alpine3.21-x86_64":  {"alpine3.21-x86_64-%", "x86_64-%"},
					"alpine3.21-ppc64le": {"alpine3.21-ppc64le-%", "ppc64le-%"},
				},
			},
			wantGroups: []ImageIndex{
				{
					Tags: []string{"%"},
					Components: []string{
						"alpine3.21-arm",
						"alpine3.21-arm64",
						"alpine3.21-ppc64le",
						"alpine3.21-x86_64",
					},
				},
				{
					Tags: []string{"alpine3.21-%"},
					Components: []string{
						"alpine3.21-arm",
						"alpine3.21-arm64",
						"alpine3.21-ppc64le",
						"alpine3.21-x86_64",
					},
				},
			},
		},
		{
			// Fuller example, including multiple alpine and windows flavors, to
			// test full grouping logic.
			name: "alpine3.21 as default with windows",
			manifest: Manifest{
				Default: map[string][]string{
					"alpine3.21-arm64":                   {"alpine3.21-arm64-%", "arm64-%"},
					"alpine3.21-arm":                     {"alpine3.21-arm-%", "arm-%"},
					"alpine3.21-x86_64":                  {"alpine3.21-x86_64-%", "x86_64-%"},
					"alpine3.21-ppc64le":                 {"alpine3.21-ppc64le-%", "ppc64le-%"},
					"alpine-latest-arm64":                {"alpine-latest-arm64-%"},
					"alpine-latest-arm":                  {"alpine-latest-arm-%"},
					"alpine-latest-x86_64":               {"alpine-latest-x86_64-%"},
					"alpine-latest-ppc64le":              {"alpine-latest-ppc64le-%"},
					"windows-servercore-ltsc2019-x86_64": {"x86_64-%-servercore1809"},
					"windows-servercore-ltsc2022-x86_64": {"x86_64-%-servercore21H2"},
					"windows-nanoserver-ltsc2019-x86_64": {"x86_64-%-nanoserver1809"},
					"windows-nanoserver-ltsc2022-x86_64": {"x86_64-%-nanoserver21H2"},
				},
			},
			wantGroups: []ImageIndex{
				// Includes only alpine3.21 and the appropriate windows images
				{
					Tags: []string{"%"},
					Components: []string{
						"alpine3.21-arm",
						"alpine3.21-arm64",
						"alpine3.21-ppc64le",
						"alpine3.21-x86_64",
						"windows-nanoserver-ltsc2019-x86_64",
						"windows-nanoserver-ltsc2022-x86_64",
					},
				},
				{
					Tags: []string{"%-nanoserver"},
					Components: []string{
						"windows-nanoserver-ltsc2019-x86_64",
						"windows-nanoserver-ltsc2022-x86_64",
					},
				},
				{
					Tags: []string{"%-servercore"},
					Components: []string{
						"windows-servercore-ltsc2019-x86_64",
						"windows-servercore-ltsc2022-x86_64",
					},
				},
				{
					Tags: []string{"alpine-latest-%"},
					Components: []string{
						"alpine-latest-arm",
						"alpine-latest-arm64",
						"alpine-latest-ppc64le",
						"alpine-latest-x86_64",
					},
				},
				// Only explicit alpine3.21, because the default arch-% has been stripped
				{
					Tags: []string{"alpine3.21-%"},
					Components: []string{
						"alpine3.21-arm",
						"alpine3.21-arm64",
						"alpine3.21-ppc64le",
						"alpine3.21-x86_64",
					},
				},
			},
		},
		{
			// Verifies that multiple tags that _don't_ strip down to just "%" are
			// included in a single ImageIndex
			name: "alpine3.21 pwsh variant",
			manifest: Manifest{
				Default: map[string][]string{
					"alpine3.21-x86_64-pwsh": {"alpine3.21-x86_64-%-pwsh", "x86_64-%-pwsh"},
				},
			},
			wantGroups: []ImageIndex{
				{
					Tags:       []string{"%-pwsh", "alpine3.21-%-pwsh"},
					Components: []string{"alpine3.21-x86_64-pwsh"},
				},
			},
		},
		{
			// Verifies handling for another flavor syntax, ensuring we don't have
			// issues with the flavor including hyphens.
			name: "alpine-edge non-default",
			manifest: Manifest{
				Default: map[string][]string{
					"alpine-edge-arm64":  {"alpine-edge-arm64-%"},
					"alpine-edge-x86_64": {"alpine-edge-x86_64-%"},
				},
			},
			wantGroups: []ImageIndex{
				{
					Tags:       []string{"alpine-edge-%"},
					Components: []string{"alpine-edge-arm64", "alpine-edge-x86_64"},
				},
			},
		},
		{
			// Verifies handling of nanoserver images, where the tag name strips
			// not just architecture, but also the windows version component.
			name: "windows-nanoserver images",
			manifest: Manifest{
				Default: map[string][]string{
					"windows-nanoserver-ltsc2019-x86_64": {"x86_64-%-nanoserver2019"},
					"windows-nanoserver-ltsc2022-x86_64": {"x86_64-%-nanoserver2022"},
				},
			},
			wantGroups: []ImageIndex{
				{
					Tags:       []string{"%"},
					Components: []string{"windows-nanoserver-ltsc2019-x86_64", "windows-nanoserver-ltsc2022-x86_64"},
				},
				{
					Tags:       []string{"%-nanoserver"},
					Components: []string{"windows-nanoserver-ltsc2019-x86_64", "windows-nanoserver-ltsc2022-x86_64"},
				},
			},
		},
		{
			// Verifies handling of servercore images, where the tag name strips
			// not just architecture, but also the windows version component.
			// Additionally, since the current logic deems servercore the "default"
			// windows variant, this  config would also push a "%" tag with the same
			// components.
			name: "windows-servercore images",
			manifest: Manifest{
				Default: map[string][]string{
					"windows-servercore-ltsc2019-x86_64": {"x86_64-%-servercore2019"},
					"windows-servercore-ltsc2022-x86_64": {"x86_64-%-servercore2022"},
				},
			},
			wantGroups: []ImageIndex{
				{
					Tags:       []string{"%-servercore"},
					Components: []string{"windows-servercore-ltsc2019-x86_64", "windows-servercore-ltsc2022-x86_64"},
				},
			},
		},
		{
			// Verifies handling for ubuntu flavors with and without pwsh variant.
			name: "mixed default and non-default in same flavor",
			manifest: Manifest{
				Default: map[string][]string{
					"ubuntu-arm64":      {"ubuntu-arm64-%"},
					"ubuntu-x86_64":     {"ubuntu-x86_64-%"},
					"ubuntu-arm64-pwsh": {"ubuntu-arm64-%-pwsh"},
				},
			},
			wantGroups: []ImageIndex{
				{
					Tags:       []string{"ubuntu-%"},
					Components: []string{"ubuntu-arm64", "ubuntu-x86_64"},
				},
				{
					Tags:       []string{"ubuntu-%-pwsh"},
					Components: []string{"ubuntu-arm64-pwsh"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndexes := GenerateIndexes(&tt.manifest)

			if diff := cmp.Diff(tt.wantGroups, gotIndexes); diff != "" {
				t.Errorf("collectIndexes() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
