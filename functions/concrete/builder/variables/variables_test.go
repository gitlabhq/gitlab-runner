//go:build !integration

package variables

import "testing"

// testProvider implements Provider backed by a simple map.
type testProvider map[string]string

func (s testProvider) Get(key string) string       { return s[key] }
func (s testProvider) ExpandValue(v string) string { return v }

func TestDefault(t *testing.T) {
	for _, tt := range []struct {
		name    string
		key     string
		def     string
		want    string
		vars    testProvider
		allowed []string
	}{
		{"present", "k", "fallback", "val", testProvider{"k": "val"}, nil},
		{"missing", "k", "fallback", "fallback", testProvider{}, nil},
		{"empty value", "k", "fallback", "fallback", testProvider{"k": ""}, nil},
		{"allowed present and valid", "k", "fallback", "val", testProvider{"k": "val"}, []string{"val", "other"}},
		{"allowed present but invalid", "k", "fallback", "fallback", testProvider{"k": "nope"}, []string{"val", "other"}},
		{"allowed missing", "k", "fallback", "fallback", testProvider{}, []string{"val", "other"}},
		{"allowed empty value", "k", "fallback", "fallback", testProvider{"k": ""}, []string{"val", "other"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Default(tt.vars, tt.key, tt.def, tt.allowed...); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultBool(t *testing.T) {
	for _, tt := range []struct {
		name string
		vars testProvider
		key  string
		def  bool
		want bool
	}{
		{"true", testProvider{"k": "true"}, "k", false, true},
		{"false", testProvider{"k": "false"}, "k", true, false},
		{"1", testProvider{"k": "1"}, "k", false, true},
		{"0", testProvider{"k": "0"}, "k", true, false},
		{"missing uses default true", testProvider{}, "k", true, true},
		{"missing uses default false", testProvider{}, "k", false, false},
		{"unparseable uses default", testProvider{"k": "nope"}, "k", true, true},
		{"empty uses default", testProvider{"k": ""}, "k", true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultBool(tt.vars, tt.key, tt.def); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultIntClamp(t *testing.T) {
	for _, tt := range []struct {
		name        string
		vars        testProvider
		key         string
		def, lo, hi int
		want        int
	}{
		{"in range", testProvider{"k": "5"}, "k", 0, 1, 10, 5},
		{"below min", testProvider{"k": "-5"}, "k", 0, 1, 10, 1},
		{"above max", testProvider{"k": "99"}, "k", 0, 1, 10, 10},
		{"at min", testProvider{"k": "1"}, "k", 0, 1, 10, 1},
		{"at max", testProvider{"k": "10"}, "k", 0, 1, 10, 10},
		{"missing clamps default", testProvider{}, "k", 50, 1, 10, 10},
		{"missing default in range", testProvider{}, "k", 5, 1, 10, 5},
		{"missing default below min", testProvider{}, "k", -1, 1, 10, 1},
		{"unparseable uses default", testProvider{"k": "abc"}, "k", 7, 1, 10, 7},
		{"unparseable default clamped", testProvider{"k": "abc"}, "k", 99, 1, 10, 10},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultIntClamp(tt.vars, tt.key, tt.def, tt.lo, tt.hi); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
