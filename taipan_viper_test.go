package taipan

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hierynomus/go-testenv"
	"github.com/hierynomus/taipan/internal/test"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestViperUnbound(t *testing.T) {
	var tests map[string]interface{} = map[string]interface{}{
		"RootShouldBeEmptyWhenNotBound":       []string{},
		"NestedShouldBeEmptyWhenNotBound":     []string{"bar"},
		"PersistentShouldBeEmptyWhenNotBound": []string{"baz"},
	}

	for name, cmd := range tests {
		cmd := cmd
		t.Run(name, func(t *testing.T) {
			rootCmd := buildViperCommand(t, "")
			rootCmd.SetArgs(cmd.([]string))
			assert.NoError(t, rootCmd.Execute())
		})
	}
}

func TestViperBindEnvVar(t *testing.T) {
	tests := map[string]struct {
		envMap   map[string]string
		expected string
		cmd      []string
	}{
		"RootShouldBindEnvVar":                  {test.AsMap("TP_FLAG", "testing-value"), "testing-value", []string{}},
		"PersistentShouldBindEnvVarOnRootLevel": {test.AsMap("TP_ONROOT", "testing-value"), "testing-value", []string{"baz"}},
		// "PersistentShouldBindEnvVarOnNestedLevel":   {test.AsMap("TP_BAZ_ONROOT", "testing-value"), "testing-value", []string{"baz"}},
		// "PersistentShouldPreferEnvVarOnNestedLevel": {test.AsMap("TP_ONROOT", "root-value", "TP_BAZ_ONROOT", "testing-value"), "testing-value", []string{"baz"}},
		"NestedShouldBindEnvVarOnNestedLevel":    {test.AsMap("TP_BAR_PARAM", "test-value"), "test-value", []string{"bar"}},
		"NestedShouldPreferEnvVarOnDeepestLevel": {test.AsMap("TP_PARAM", "root-value", "TP_BAR_PARAM", "test-value"), "test-value", []string{"bar"}},
	}

	for name, data := range tests {
		ctx := log.Logger.WithContext(context.Background())
		data := data
		t.Run(name, func(t *testing.T) {
			defer testenv.PatchEnv(t, data.envMap)()
			rootCmd := buildViperCommand(t, data.expected)
			rootCmd.SetArgs(data.cmd)

			assert.NoError(t, rootCmd.ExecuteContext(ctx))
		})
	}
}

func TestViperBindConfigFile(t *testing.T) {
	tests := map[string]struct {
		contents string
		expected string
		cmd      []string
	}{
		"RootShouldBindConfigFile":                   {"flag: a-value\n", "a-value", []string{}},
		"PersistentShouldBindConfigFileOnRootLevel":  {"onroot: testing-value\n", "testing-value", []string{"baz"}},
		"NestedShouldBindConfigFileOnNestedLevel":    {"bar:\n  param: test-value\n", "test-value", []string{"bar"}},
		"NestedShouldPreferConfigFileOnDeepestLevel": {"param: root-value\nbar:\n  param: test-value", "test-value", []string{"bar"}},
	}

	for name, data := range tests {
		ctx := log.Logger.WithContext(context.Background())
		d := data
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, ioutil.WriteFile("unittest-taipan.yaml", []byte(d.contents), 0600))
			defer os.Remove("unittest-taipan.yaml")
			rootCmd := buildViperCommand(t, d.expected)
			rootCmd.SetArgs(d.cmd)

			assert.NoError(t, rootCmd.ExecuteContext(ctx))
		})
	}
}

type TestCfg struct {
	Flag   string `yaml:"flag"`
	OnRoot string `yaml:"onroot"`
	Bar    Bar    `yaml:"bar"`
}

type Bar struct {
	Param string `yaml:"param"`
}

func buildViperCommand(t *testing.T, expectedValue string) *cobra.Command {
	cfg := &TestCfg{}

	rootCmd := &cobra.Command{
		Use: "foo",
		Run: func(cmd *cobra.Command, args []string) {
			assert.Equal(t, expectedValue, cfg.Flag)
		},
	}

	rootCmd.Flags().String("flag", "", "")
	rootCmd.PersistentFlags().String("onroot", "", "")

	barCmd := &cobra.Command{
		Use: "bar",
		Run: func(cmd *cobra.Command, args []string) {
			assert.Equal(t, expectedValue, cfg.Bar.Param)
		},
	}

	bazCmd := &cobra.Command{
		Use: "baz",
		Run: func(cmd *cobra.Command, args []string) {
			assert.Equal(t, expectedValue, cfg.OnRoot)
		},
	}

	barCmd.Flags().String("param", "", "")
	rootCmd.AddCommand(barCmd)
	rootCmd.AddCommand(bazCmd)

	tai := New(&Config{
		DefaultConfigName:  "unittest-taipan",
		EnvironmentPrefix:  "TP",
		PrefixCommands:     true,
		ConfigurationPaths: []string{"."},
		ConfigObject:       cfg,
	})
	tai.Inject(rootCmd)

	return rootCmd
}
