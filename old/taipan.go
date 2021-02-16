package taipan

import (
	"fmt"
	"reflect"
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
	ConfigObject       interface{}
}

type Taipan struct {
	v      *viper.Viper
	config *Config
}

func New(config *Config) *Taipan {
	return &Taipan{
		config: config,
	}
}

func (t *Taipan) init(configFile string) error {
	if t.v != nil {
		return nil
	}

	v := viper.New()

	if configFile == "" {
		v.SetConfigName(t.config.DefaultConfigName)
		for _, p := range t.config.ConfigurationPaths {
			v.AddConfigPath(p)
		}
	} else {
		v.SetConfigFile(configFile)
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

	t.v = v

	return nil
}

func (t *Taipan) unmarshalConfigObject() error {
	obj := t.config.ConfigObject
	if obj == nil {
		return nil
	}

	ri := reflect.TypeOf(obj)
	if ri.Kind() != reflect.Ptr {
		return fmt.Errorf("cannot unmarshall into a non-pointer: %s", ri.Name())
	}

	return t.v.Unmarshal(obj)
}

// Inject Taipan into the (root) command
func (t *Taipan) Inject(cmd *cobra.Command) {
	f := EmptyFunc
	if cmd.PersistentPreRunE != nil {
		f = cmd.PersistentPreRunE
	}
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		configFile, _ := cmd.Flags().GetString("config")
		if err := t.init(configFile); err != nil {
			return err
		}

		// // Bind the flags to the viper values
		// if err := t.bindPrefixedViperToFlags(cmd); err != nil {
		// 	return err
		// }

		// Bind the viper to the current command's flags
		if err := t.bindViperToFlags(cmd); err != nil {
			return err
		}

		if err := t.unmarshalConfigObject(); err != nil {
			return err
		}

		return f(cmd, args)
	}

	if t.config.AddConfigFlag {
		cmd.Flags().StringP("config", "c", "", "Configuration file to use")
	}
}

func (t *Taipan) bindViperToFlags(cmd *cobra.Command) error {
	for c := cmd; c != nil; c = c.Parent() {
		namePrefix := prefix(c)
		if err := t.bindPrefixedViperToFlags(cmd, namePrefix); err != nil {
			return err
		}
	}

	return nil
}

func (t *Taipan) bindPrefixedViperToFlags(cmd *cobra.Command, flagPrefix string) error {
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
		collector.Collect(t.v.BindEnv(viperName, fmt.Sprintf("%s_%s", t.config.EnvironmentPrefix, envVarSuffix)))

		collector.Collect(t.v.BindPFlag(viperName, f))
		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && t.v.IsSet(viperName) {
			val := t.v.Get(viperName)
			collector.Collect(cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val)))
		}
	})

	if collector.HasErrors() {
		return collector
	}

	if cmd.Parent() == nil {
		return nil
	}

	return t.bindPrefixedViperToFlags(cmd.Parent(), flagPrefix)
}

// prefix returns the prefix for the command, this is _not_ including the Root command name
func prefix(c *cobra.Command) string {
	if c.Parent() == nil {
		return ""
	}

	namePrefix := c.Name()

	for r := c.Parent(); r.Parent() != nil; r = r.Parent() {
		namePrefix = fmt.Sprintf("%s.%s", r.Name(), namePrefix)
	}

	return namePrefix
}
