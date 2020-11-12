package taipan

import (
	"fmt"
	"strings"

	"github.com/hierynomus/gotils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var EmptyFunc = func(cmd *cobra.Command, args []string) error {
	return nil
}

type Config struct {
	DefaultConfigName  string
	ConfigurationPaths []string
	EnvironmentPrefix  string
	AddConfigFlag      bool
}

type Taipan struct {
	config *Config
}

func New(config *Config) *Taipan {
	return &Taipan{
		config: config,
	}
}

// Inject Taipan into the (root) command
func (t *Taipan) Inject(cmd *cobra.Command) {
	f := EmptyFunc
	if cmd.PersistentPreRunE != nil {
		f = cmd.PersistentPreRunE
	}
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := t.injectConfig(cmd); err != nil {
			return err
		}

		return f(cmd, args)
	}

	if t.config.AddConfigFlag {
		cmd.Flags().StringP("config", "c", "", "Configuration file to use")
	}
}

func (t *Taipan) injectConfig(cmd *cobra.Command) error {
	v := viper.New()

	if f, _ := cmd.Flags().GetString("config"); f != "" {
		v.SetConfigFile(f)
	} else {
		v.SetConfigName(t.config.DefaultConfigName)
		for _, p := range t.config.ConfigurationPaths {
			v.AddConfigPath(p)
		}
	}

	// Attempt to read the config file, gracefully ignoring errors
	// caused by a config file not being found. Return an error
	// if we cannot parse the config file.
	if err := v.ReadInConfig(); err != nil {
		// It's okay if there isn't a config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	// When we bind flags to environment variables expect that the
	// environment variables are prefixed, e.g. a flag like --number
	// binds to an environment variable STING_NUMBER. This helps
	// avoid conflicts.
	v.SetEnvPrefix(t.config.EnvironmentPrefix)

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Bind to environment variables
	// Works great for simple config names, but needs help for names
	// like --favorite-color which we fix in the bindFlags function
	v.AutomaticEnv()

	// Bind the current command's flags to viper
	return t.bindFlags(cmd, v)
}

func (t *Taipan) bindFlags(cmd *cobra.Command, v *viper.Viper) error {
	for c := cmd; c != nil; c = c.Parent() {
		namePrefix := prefix(c)
		if err := t.bindPrefixedFlags(cmd, namePrefix, v); err != nil {
			return err
		}
	}

	return nil
}

func (t *Taipan) bindPrefixedFlags(cmd *cobra.Command, flagPrefix string, v *viper.Viper) error {
	collector := &gotils.ErrCollector{}

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		viperName := f.Name
		if flagPrefix != "" {
			viperName = fmt.Sprintf("%s.%s", flagPrefix, viperName)
		}

		envVarSuffix := viperName
		if strings.ContainsAny(viperName, "-.") {
			envVarSuffix = strings.NewReplacer("-", "_", ".", "_").Replace(viperName)
		}

		envVarSuffix = strings.ToUpper(envVarSuffix)
		collector.Collect(v.BindEnv(viperName, fmt.Sprintf("%s_%s", t.config.EnvironmentPrefix, envVarSuffix)))

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(viperName) {
			val := v.Get(viperName)
			collector.Collect(cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)))
		}
	})

	if collector.HasErrors() {
		return collector
	}

	if cmd.Parent() == nil {
		return nil
	}

	return t.bindPrefixedFlags(cmd.Parent(), flagPrefix, v)
}

// prefix returns the prefix for the command, this is _not_ including the Root command name
func prefix(c *cobra.Command) string {
	if c.Parent() == nil {
		return ""
	}

	namePrefix := c.Name()

	for t := c.Parent(); t.Parent() != nil; t = t.Parent() {
		namePrefix = fmt.Sprintf("%s.%s", t.Name(), namePrefix)
	}

	return namePrefix
}
