//go:build !integration

package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func withMapping(t *testing.T, schema, dualEmit int) {
	t.Helper()
	prev := currentFieldMapping.Load()
	SetFieldMapping(schema, dualEmit)
	t.Cleanup(func() {
		currentFieldMapping.Store(prev)
	})
}

func TestDeprecatedField(t *testing.T) {
	tests := map[string]struct {
		schema       int
		dualEmit     int
		alias        string
		value        any
		expectedKeys []string
		missingKeys  []string
	}{
		"schema 0: emits deprecated alias only": {
			schema:       0,
			dualEmit:     0,
			alias:        "organization_id",
			value:        42,
			expectedKeys: []string{"organization_id"},
			missingKeys:  []string{"gl_organization_id"},
		},
		"schema 1: emits standard name only": {
			schema:       1,
			dualEmit:     0,
			alias:        "organization_id",
			value:        42,
			expectedKeys: []string{"gl_organization_id"},
			missingKeys:  []string{"organization_id"},
		},
		"dual emit (0, 1): emits both": {
			schema:       0,
			dualEmit:     1,
			alias:        "organization_id",
			value:        42,
			expectedKeys: []string{"organization_id", "gl_organization_id"},
		},
		"unknown field: passes through unchanged": {
			schema:       1,
			dualEmit:     0,
			alias:        "custom_field",
			value:        "val",
			expectedKeys: []string{"custom_field"},
		},
		"error field schema 0": {
			schema:       0,
			dualEmit:     0,
			alias:        "error",
			value:        "something broke",
			expectedKeys: []string{"error"},
			missingKeys:  []string{"error_message"},
		},
		"error field schema 1": {
			schema:       1,
			dualEmit:     0,
			alias:        "error",
			value:        "something broke",
			expectedKeys: []string{"error_message"},
			missingKeys:  []string{"error"},
		},
		"error field dual emit": {
			schema:       0,
			dualEmit:     1,
			alias:        "error",
			value:        "something broke",
			expectedKeys: []string{"error", "error_message"},
		},
		"component field schema 0 (variant version 0, never migrated)": {
			schema:       0,
			dualEmit:     0,
			alias:        "component",
			value:        "pause-pod-manager",
			expectedKeys: []string{"component"},
			missingKeys:  []string{"service_name"},
		},
		"component field schema 1 (variant version 0, never migrated)": {
			schema:       1,
			dualEmit:     0,
			alias:        "component",
			value:        "pause-pod-manager",
			expectedKeys: []string{"component"},
			missingKeys:  []string{"service_name"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			withMapping(t, tt.schema, tt.dualEmit)

			result := DeprecatedField(tt.alias, tt.value)

			for _, key := range tt.expectedKeys {
				assert.Contains(t, result, key, "expected key %q in result", key)
			}
			for _, key := range tt.missingKeys {
				assert.NotContains(t, result, key, "unexpected key %q in result", key)
			}
		})
	}
}

func TestDeprecatedFieldDual(t *testing.T) {
	tests := map[string]struct {
		schema        int
		dualEmit      int
		alias         string
		legacyValue   any
		standardValue any
		expectedKeys  []string
		expectedVals  map[string]any
	}{
		"duration schema 0: emits legacy value only": {
			schema:        0,
			dualEmit:      0,
			alias:         "duration",
			legacyValue:   "5s",
			standardValue: 5.0,
			expectedKeys:  []string{"duration"},
			expectedVals:  map[string]any{"duration": "5s"},
		},
		"duration schema 1: emits standard value only": {
			schema:        1,
			dualEmit:      0,
			alias:         "duration",
			legacyValue:   "5s",
			standardValue: 5.0,
			expectedKeys:  []string{"duration_s"},
			expectedVals:  map[string]any{"duration_s": 5.0},
		},
		"duration dual emit (0, 1): emits both with respective values": {
			schema:        0,
			dualEmit:      1,
			alias:         "duration",
			legacyValue:   "5s",
			standardValue: 5.0,
			expectedKeys:  []string{"duration", "duration_s"},
			expectedVals:  map[string]any{"duration": "5s", "duration_s": 5.0},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			withMapping(t, tt.schema, tt.dualEmit)

			result := DeprecatedFieldDual(tt.alias, tt.legacyValue, tt.standardValue)

			for _, key := range tt.expectedKeys {
				assert.Contains(t, result, key)
			}
			for key, val := range tt.expectedVals {
				assert.Equal(t, val, result[key])
			}
		})
	}
}

func TestAddDeprecatedField(t *testing.T) {
	withMapping(t, 0, 1)

	fields := logrus.Fields{"existing": "value"}
	AddDeprecatedField(fields, "organization_id", 42)

	assert.Equal(t, "value", fields["existing"], "pre-existing keys should be preserved")
	assert.Equal(t, 42, fields["organization_id"])
	assert.Equal(t, 42, fields["gl_organization_id"])
}

func TestAddDeprecatedFieldDual(t *testing.T) {
	withMapping(t, 0, 1)

	fields := logrus.Fields{"existing": "value"}
	AddDeprecatedFieldDual(fields, "duration", "5s", 5.0)

	assert.Equal(t, "value", fields["existing"], "pre-existing keys should be preserved")
	assert.Equal(t, "5s", fields["duration"])
	assert.Equal(t, 5.0, fields["duration_s"])
}

func TestLoadFieldMapping(t *testing.T) {
	t.Run("defaults to zero when unset", func(t *testing.T) {
		prev := currentFieldMapping.Load()
		currentFieldMapping.Store(nil)
		t.Cleanup(func() { currentFieldMapping.Store(prev) })

		assert.Equal(t, fieldMapping{}, loadFieldMapping())
	})

	t.Run("returns both settings from a single store", func(t *testing.T) {
		withMapping(t, 3, 7)

		assert.Equal(t, fieldMapping{schemaVersion: 3, dualEmitTarget: 7}, loadFieldMapping())
	})
}
