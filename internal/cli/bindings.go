package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func mustBindEnv(v *viper.Viper, key string, envvars ...string) {
	args := append([]string{key}, envvars...)
	if err := v.BindEnv(args...); err != nil {
		panic(err)
	}
}

func mustBindPFlag(v *viper.Viper, key string, flag *pflag.Flag) {
	if err := v.BindPFlag(key, flag); err != nil {
		panic(err)
	}
}

func mustMarkFlagRequired(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}
