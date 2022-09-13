package parser

import (
	"regexp"
)

// The specification of regular expression used for parsing Windows volumes
// specification was taken from:
//
// https://github.com/docker/engine/blob/a79fabbfe84117696a19671f4aa88b82d0f64fc1/volume/mounts/windows_parser.go
//
// The original source is licensed under Apache License 2.0 and the copyright for it
// goes to Docker, Inc.

//nolint:lll
const (
	// Spec should be in the format [source:]destination[:mode]
	//
	// Examples: c:\foo bar:d:rw
	//           c:\foo:d:\bar
	//           myname:d:
	//           d:\
	//
	// Explanation of this regex! Thanks @thaJeztah on IRC and gist for help. See
	// https://gist.github.com/thaJeztah/6185659e4978789fb2b2. A good place to
	// test is https://regex-golang.appspot.com/assets/html/index.html
	//
	// Useful link for referencing named capturing groups:
	// http://stackoverflow.com/questions/20750843/using-named-matches-from-go-regex
	//
	// There are three match groups: source, destination and mode.
	//

	// windowsHostDir is the first option of a source
	windowsHostDir = `(?:\\\\\?\\)?[a-z]:[\\/](?:[^\\/:*?"<>|\r\n]+[\\/]?)*`
	// windowsVolumeName is the second option of a source
	windowsVolumeName = `[^\\/:*?"<>|\r\n]+`
	// windowsNamedPipe matches a named pipe path (starts with `\\.\pipe\`, possibly with / instead of \)
	windowsNamedPipe = `[/\\]{2}\.[/\\]pipe[/\\][^:*?"<>|\r\n]+`
	// windowsSource is the combined possibilities for a source
	windowsSource = `((?P<source>((` + windowsHostDir + `)|(` + windowsVolumeName + `)|(` + windowsNamedPipe + `))):)?`

	// Source. Can be either a host directory, a name, or omitted:
	//  HostDir:
	//    -  Essentially using the folder solution from
	//       https://www.safaribooksonline.com/library/view/regular-expressions-cookbook/9781449327453/ch08s18.html
	//       but adding case insensitivity.
	//    -  Must be an absolute path such as c:\path
	//    -  Can include spaces such as `c:\program files`
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional
	//  Name:
	//    -  Must not contain invalid NTFS filename characters (https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx)
	//    -  And then followed by a colon which is not in the capture group
	//    -  And can be optional

	// windowsDestination is the regex expression for the mount destination
	windowsDestination = `(?P<destination>((?:\\\\\?\\)?([a-z]):((?:[\\/][^\\/:*?"<>\r\n]+)*[\\/]?))|(` + windowsNamedPipe + `))`

	// windowsMode is the regex expression for the mode of the mount
	// Mode (optional):
	//    -  Hopefully self explanatory in comparison to above regex's.
	//    -  Colon is not in the capture group
	windowsMode = `(:(?P<mode>(?i)ro|rw))?`
)

type windowsParser struct {
	baseParser
}

func NewWindowsParser() Parser {
	return &windowsParser{
		baseParser: baseParser{
			path: newWindowsPath(),
		},
	}
}

func (p *windowsParser) ParseVolume(spec string) (*Volume, error) {
	specExp := regexp.MustCompile(`(?i)^` + windowsSource + windowsDestination + windowsMode + `$`)

	parts, err := p.matchesToVolumeSpecParts(spec, specExp)
	if err != nil {
		return nil, err
	}

	return newVolume(parts["source"], parts["destination"], parts["mode"], "", ""), nil
}
