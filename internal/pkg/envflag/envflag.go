// Package envflag binds environment variables to cobra flags.
// If a flag is not set explicitly, the env var value is used as default.
package envflag

import (
	"os"

	"github.com/spf13/cobra"
)

// Bind sets the default value of a flag from an environment variable.
// Call this in PersistentPreRun or PreRun, before flag values are used.
// Explicit flag values always take precedence over env vars.
func Bind(cmd *cobra.Command, flagName, envVar string) {
	if v, ok := os.LookupEnv(envVar); ok {
		f := cmd.Flags().Lookup(flagName)
		if f != nil && !f.Changed {
			_ = f.Value.Set(v)
		}
	}
}

// BindAll binds multiple flag→env pairs.
func BindAll(cmd *cobra.Command, bindings map[string]string) {
	for flagName, envVar := range bindings {
		Bind(cmd, flagName, envVar)
	}
}
