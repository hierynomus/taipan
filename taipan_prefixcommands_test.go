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

type RootConfig struct {
	NounConfig NounConfig `yaml:"noun"`
}
type NounConfig struct {
	NounFlag string `yaml:"nounflag"`
	VerbFlag string `yaml:"verb-flag"`
}

func TestShouldCalculatePrefixes(t *testing.T) {
	r := cmdTree()
	ps := prefix(r.Commands()[0].Commands()[0])
	assert.Contains(t, ps, "noun.verb")
	assert.Contains(t, ps, "noun")
}

func TestTaipanFlags(t *testing.T) {
	r := cmdTree()
	cfg := &RootConfig{}
	tp := New(&Config{
		EnvironmentPrefix: "TST",
		ConfigObject:      cfg,
		PrefixCommands:    true,
	})

	tp.Inject(r)
	r.SetArgs([]string{"noun", "verb", "--nounFlag", "val1", "--verb-flag", "val2"})
	assert.NoError(t, r.Execute())

	assert.Equal(t, cfg.NounConfig.NounFlag, "val1")
	assert.Equal(t, cfg.NounConfig.VerbFlag, "val2")
}

func TestTaipanEnv(t *testing.T) {
	ctx := log.Logger.WithContext(context.Background())
	env := test.AsMap("TST_NOUN_NOUNFLAG", "val1", "TST_NOUN_VERB_FLAG", "val2")
	defer testenv.PatchEnv(t, env)()

	r := cmdTree()
	cfg := &RootConfig{}
	tp := New(&Config{
		EnvironmentPrefix: "TST",
		ConfigObject:      cfg,
		PrefixCommands:    true,
	})

	tp.Inject(r)
	r.SetArgs([]string{"noun", "verb"})
	assert.NoError(t, r.ExecuteContext(ctx))

	assert.Equal(t, cfg.NounConfig.NounFlag, "val1")
	assert.Equal(t, cfg.NounConfig.VerbFlag, "val2")
}

func TestTaipanConfig(t *testing.T) {
	ctx := log.Logger.WithContext(context.Background())
	assert.NoError(t, ioutil.WriteFile("unittest-taipan.yaml", []byte(`noun:
  nounflag: val1
  verb-flag: val2
`), 0600))
	defer os.Remove("unittest-taipan.yaml")
	r := cmdTree()
	cfg := &RootConfig{}
	tp := New(&Config{
		DefaultConfigName:  "unittest-taipan",
		ConfigurationPaths: []string{"."},
		ConfigObject:       cfg,
		PrefixCommands:     true,
	})

	tp.Inject(r)
	r.SetArgs([]string{"noun", "verb"})
	assert.NoError(t, r.ExecuteContext(ctx))

	assert.Equal(t, cfg.NounConfig.NounFlag, "val1")
	assert.Equal(t, cfg.NounConfig.VerbFlag, "val2")
}

func cmdTree() *cobra.Command {
	r := &cobra.Command{
		Use: "cmd",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	noun := &cobra.Command{
		Use: "noun",
	}

	noun.PersistentFlags().String("nounFlag", "", "")

	verb := &cobra.Command{
		Use: "verb",
		Run: func(cmd *cobra.Command, args []string) {},
	}

	verb.Flags().String("verb-flag", "", "")

	noun.AddCommand(verb)
	r.AddCommand(noun)

	return r
}
