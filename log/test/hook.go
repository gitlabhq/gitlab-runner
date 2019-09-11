package test

import (
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

// NewHook will create a new global hook that can be used for tests after which
// it will remove when the returned function invoked.
//
// This shouldn't be used when you are writing a new package/structure, you
// should instead pass the logger to that struct and add the Hook to that struct
// only, try to avoid the global logger. This has multiple benefits, for example
// having that struct with specific logger settings that doesn't effect the
// logger in another part of the application. For example:
//
// type MyNewStruct struct {
// 		logger   logrus.FieldLogger
// }
//
// The more hooks we add to the tests the more memory we are leaking.
func NewHook() (*test.Hook, func()) {
	// Copy all the previous hooks so we revert back to that state.
	oldHooks := logrus.LevelHooks{}
	for level, hooks := range logrus.StandardLogger().Hooks {
		oldHooks[level] = hooks
	}

	newHook := test.NewGlobal()
	return newHook, func() {
		logrus.StandardLogger().ReplaceHooks(oldHooks)
	}
}
