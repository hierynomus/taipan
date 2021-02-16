// Taipan binds Cobra and Viper in a standard and structured way
// A difference with using Viper directly is that any ConfigObject passed in
// will be unmarshalled using YAML (and not mapstructure)
package taipan

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/hierynomus/gotils"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var emptyFunc = func(cmd *cobra.Command, args []string) error {
	return nil
}

type Config struct {
	DefaultConfigName  string
	ConfigurationPaths []string
	EnvironmentPrefix  string
	PrefixCommands     bool
	NamespaceFlags     bool
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

func (t *Taipan) init(ctx context.Context, configFile string) error {
	if t.v != nil {
		return nil
	}

	v := viper.New()

	if configFile == "" {
		log.Ctx(ctx).Trace().Str("configFile", t.config.DefaultConfigName).Msg("Loading default config")
		v.SetConfigName(t.config.DefaultConfigName)

		for _, p := range t.config.ConfigurationPaths {
			v.AddConfigPath(p)
		}
	} else {
		log.Ctx(ctx).Trace().Str("configFile", configFile).Msg("Loading config")
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

		log.Ctx(ctx).Info().Str("configFile", v.ConfigFileUsed()).Msg("Could not find configuration file...")
	}

	// When we bind flags to environment variables expect that the
	// environment variables are prefixed, e.g. a flag like --number
	// binds to an environment variable PREFIX_NUMBER. This helps
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

func (t *Taipan) Inject(cmd *cobra.Command) {
	f := emptyFunc
	if cmd.PersistentPreRunE != nil {
		f = cmd.PersistentPreRunE
	}

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		configFile, _ := cmd.Flags().GetString("config")
		if err := t.init(ctx, configFile); err != nil {
			return err
		}

		// Bind to all cmd Flags
		if err := t.bindFlags(ctx, cmd); err != nil {
			return err
		}

		// Bind all env variables with right prefix
		if err := t.bindEnv(ctx); err != nil {
			return err
		}

		if err := t.unmarshalConfigObject(ctx); err != nil {
			return err
		}

		return f(cmd, args)
	}

	if t.config.AddConfigFlag {
		cmd.PersistentFlags().StringP("config", "c", "", "Configuration file to use")
	}
}

// bindFlags will bind the flags to the Viper keys, changing `-` to `.` so that
// a flag such as `--keycloak-username` ends up as `keycloak.username`, effectively
// nesting the `username` under the `keycloak` namespace
func (t *Taipan) bindFlags(ctx context.Context, cmd *cobra.Command) error {
	collector := &gotils.ErrCollector{}
	b := func(flag *pflag.Flag, name string) {
		log.Ctx(ctx).Trace().Str("flag", flag.Name).Str("viper-name", name).Msg("Binding flag")
		collector.Collect(t.v.BindPFlag(name, flag))
	}

	replacer := strings.NewReplacer("-", ".", "_", ".")
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		name := replacer.Replace(f.Name)
		b(f, name)

		if t.config.PrefixCommands {
			prefix := prefix(cmd)
			for _, p := range prefix {
				alias := fmt.Sprintf("%s.%s", p, name)
				b(f, alias)
				// log.Ctx(ctx).Trace().Str("viper-name", name).Str("alias", alias).Msg("Register alias")
				// t.v.RegisterAlias(alias, name)
			}
		}
	})

	if collector.HasErrors() {
		return collector
	}

	return nil
}

func (t *Taipan) bindEnv(ctx context.Context) error {
	if t.config.EnvironmentPrefix == "" {
		log.Ctx(ctx).Trace().Msg("Skipping environment, no prefix configured")
		return nil
	}

	envPrefix := fmt.Sprintf("%s_", t.config.EnvironmentPrefix)
	repl := strings.NewReplacer("_", ".")
	for k := range envMap(os.Environ()) {
		if !strings.HasPrefix(k, envPrefix) {
			continue
		}

		trimmed := strings.TrimPrefix(k, envPrefix)
		viperKey := strings.ToLower(repl.Replace(trimmed))

		log.Ctx(ctx).Trace().Str("viper-name", viperKey).Str("env-key", k).Msg("Binding environment")
		if err := t.v.BindEnv(viperKey, k); err != nil {
			return err
		}
	}

	return nil
}

func (t *Taipan) unmarshalConfigObject(_ context.Context) error {
	obj := t.config.ConfigObject
	if obj == nil {
		return nil
	}

	ri := reflect.TypeOf(obj)
	if ri.Kind() != reflect.Ptr {
		return fmt.Errorf("cannot unmarshall into a non-pointer: %s", ri.Name())
	}

	settings := t.v.AllSettings()
	b, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(b, obj)
}

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v := splitEnvKeyValue(kv)
		m[k] = v
	}

	return m
}

func splitEnvKeyValue(kv string) (string, string) {
	switch {
	case kv == "":
		return "", ""
	case strings.HasPrefix(kv, "="):
		k, v := splitEnvKeyValue(kv[1:])
		return "=" + k, v
	case strings.Contains(kv, "="):
		parts := strings.SplitN(kv, "=", 2)
		return parts[0], parts[1]
	default:
		return kv, ""
	}
}

// prefix returns the prefix for the command, this is _not_ including the Root command name
func prefix(c *cobra.Command) []string {
	prefixes := []string{}
	if c.Parent() == nil {
		return prefixes
	}

	p := prefix(c.Parent())
	if len(p) == 0 {
		prefixes = append(prefixes, c.Name())
		return prefixes
	}

	prefixes = append(prefixes, p...)
	prefixes = append(prefixes, fmt.Sprintf("%s.%s", p[len(p)-1], c.Name()))

	return prefixes
}
