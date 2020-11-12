package taipan

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hierynomus/go-testenv"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestUnbound(t *testing.T) {
	var tests map[string]interface{} = map[string]interface{}{
		"RootShouldBeEmptyWhenNotBound":       []string{},
		"NestedShouldBeEmptyWhenNotBound":     []string{"bar"},
		"PersistentShouldBeEmptyWhenNotBound": []string{"baz"},
	}

	for name, cmd := range tests {
		cmd := cmd
		t.Run(name, func(t *testing.T) {
			rootCmd := buildCommand(t, "")
			rootCmd.SetArgs(cmd.([]string))
			assert.NoError(t, rootCmd.Execute())
		})
	}
}

func TestBindEnvVar(t *testing.T) {
	tests := map[string]struct {
		envMap   map[string]string
		expected string
		cmd      []string
	}{
		"RootShouldBindEnvVar":                      {asMap("TP_FLAG", "testing-value"), "testing-value", []string{}},
		"PersistentShouldBindEnvVarOnRootLevel":     {asMap("TP_ONROOT", "testing-value"), "testing-value", []string{"baz"}},
		"PersistentShouldBindEnvVarOnNestedLevel":   {asMap("TP_BAZ_ONROOT", "testing-value"), "testing-value", []string{"baz"}},
		"PersistentShouldPreferEnvVarOnNestedLevel": {asMap("TP_ONROOT", "root-value", "TP_BAZ_ONROOT", "testing-value"), "testing-value", []string{"baz"}},
		"NestedShouldBindHigherLevelEnvVar":         {asMap("TP_PARAM", "root-value"), "root-value", []string{"bar"}},
		"NestedShouldBindEnvVarOnNestedLevel":       {asMap("TP_BAR_PARAM", "test-value"), "test-value", []string{"bar"}},
		"NestedShouldPreferEnvVarOnDeepestLevel":    {asMap("TP_PARAM", "root-value", "TP_BAR_PARAM", "test-value"), "test-value", []string{"bar"}},
	}

	for name, data := range tests {
		data := data
		t.Run(name, func(t *testing.T) {
			defer testenv.PatchEnv(t, data.envMap)()
			rootCmd := buildCommand(t, data.expected)
			rootCmd.SetArgs(data.cmd)

			assert.NoError(t, rootCmd.Execute())
		})
	}
}

func TestBindConfigFile(t *testing.T) {
	tests := map[string]struct {
		contents string
		expected string
		cmd      []string
	}{
		"RootShouldBindConfigFile":                      {"flag: a-value\n", "a-value", []string{}},
		"PersistentShouldBindConfigFileOnRootLevel":     {"onroot: testing-value\n", "testing-value", []string{"baz"}},
		"PersistentShouldBindConfigFileOnNestedLevel":   {"baz:\n  onroot: testing-value\n", "testing-value", []string{"baz"}},
		"PersistentShouldPreferConfigFileOnNestedLevel": {"onroot: root-value \nbaz:\n  onroot: testing-value", "testing-value", []string{"baz"}},
		"NestedShouldBindHigherLevelConfigFile":         {"param: root-value\n", "root-value", []string{"bar"}},
		"NestedShouldBindConfigFileOnNestedLevel":       {"bar:\n  param: test-value\n", "test-value", []string{"bar"}},
		"NestedShouldPreferConfigFileOnDeepestLevel":    {"param: root-value\nbar:\n  param: test-value", "test-value", []string{"bar"}},
	}

	for name, data := range tests {
		d := data
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, ioutil.WriteFile("unittest-taipan.yaml", []byte(d.contents), 0600))
			defer os.Remove("unittest-taipan.yaml")
			rootCmd := buildCommand(t, d.expected)
			rootCmd.SetArgs(d.cmd)

			assert.NoError(t, rootCmd.Execute())
		})
	}
}

func buildCommand(t *testing.T, expectedValue string) *cobra.Command {
	var param string
	var rootFlag string
	var persistentFlag string
	rootCmd := &cobra.Command{
		Use: "foo",
		Run: func(cmd *cobra.Command, args []string) {
			assert.Equal(t, expectedValue, rootFlag)
		},
	}

	rootCmd.Flags().StringVar(&rootFlag, "flag", "", "")
	rootCmd.PersistentFlags().StringVar(&persistentFlag, "onroot", "", "")

	barCmd := &cobra.Command{
		Use: "bar",
		Run: func(cmd *cobra.Command, args []string) {
			assert.Equal(t, expectedValue, param)
		},
	}

	bazCmd := &cobra.Command{
		Use: "baz",
		Run: func(cmd *cobra.Command, args []string) {
			assert.Equal(t, expectedValue, persistentFlag)
		},
	}

	barCmd.Flags().StringVar(&param, "param", "", "")
	rootCmd.AddCommand(barCmd)
	rootCmd.AddCommand(bazCmd)

	tai := New(&Config{
		DefaultConfigName:  "unittest-taipan",
		EnvironmentPrefix:  "TP",
		ConfigurationPaths: []string{"."},
	})
	tai.Inject(rootCmd)

	return rootCmd
}

func asMap(kv ...string) map[string]string {
	if len(kv)%2 != 0 {
		panic(fmt.Errorf("cannot convert to Map"))
	}

	m := make(map[string]string, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k := kv[i]
		v := kv[i+1]
		m[k] = v
	}

	return m
}
