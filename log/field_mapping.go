package log

import (
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/labkit/v2/fields"
)

// fieldMapping is the log field migration setting pair. It is immutable once
// stored, mirroring labkit's fields.ServiceMapping.
type fieldMapping struct {
	schemaVersion  int
	dualEmitTarget int
}

// currentFieldMapping is read on the hot logging path (see
// DeprecatedFieldDual) and written on config reload via SetFieldMapping, which
// run in different goroutines. Both settings are swapped as a single pointer so
// a reader can never observe one of them updated without the other, which
// would be a combination Config.validateLogFieldSettings itself rejects.
var currentFieldMapping atomic.Pointer[fieldMapping]

// SetFieldMapping sets the log field schema version and dual emit target used
// to decide whether a deprecated field is emitted under its legacy alias, its
// standard name, or both.
func SetFieldMapping(schemaVersion, dualEmitTarget int) {
	currentFieldMapping.Store(&fieldMapping{
		schemaVersion:  schemaVersion,
		dualEmitTarget: dualEmitTarget,
	})
}

// loadFieldMapping returns the current settings, defaulting both to 0 until
// SetFieldMapping has run.
func loadFieldMapping() fieldMapping {
	if m := currentFieldMapping.Load(); m != nil {
		return *m
	}

	return fieldMapping{}
}

func DeprecatedField(alias string, value any) logrus.Fields {
	return DeprecatedFieldDual(alias, value, value)
}

// Temporary: when adopting labkit/v2 log, replace this with the LabKit equivalent.
func DeprecatedFieldDual(alias string, legacyValue, standardValue any) logrus.Fields {
	logFields := make(logrus.Fields, 2)
	AddDeprecatedFieldDual(logFields, alias, legacyValue, standardValue)

	return logFields
}

// AddDeprecatedField adds the deprecated logField(s) for alias to the destination
// map in place, avoiding a manual merge at the call site.
func AddDeprecatedField(logFields logrus.Fields, alias string, value any) {
	AddDeprecatedFieldDual(logFields, alias, value, value)
}

// AddDeprecatedFieldDual adds the deprecated field(s) for alias to the
// destination map in place, avoiding a manual merge at the call site.
func AddDeprecatedFieldDual(logFields logrus.Fields, alias string, legacyValue, standardValue any) {
	d, isLegacy := fields.Deprecated[alias]
	if !isLegacy {
		logFields[alias] = legacyValue
		return
	}

	standard := d.Standard
	version := fields.VariantVersion[standard]
	mapping := loadFieldMapping()

	if version != 0 && mapping.schemaVersion >= version {
		logFields[standard] = standardValue
		return
	}

	logFields[alias] = legacyValue
	if version != 0 && mapping.dualEmitTarget >= version {
		logFields[standard] = standardValue
	}
}
