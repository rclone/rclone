package configmap_test

import (
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleString(t *testing.T) {
	for _, tt := range []struct {
		name string
		want string
		in   configmap.Simple
	}{
		{name: "Nil", want: "", in: configmap.Simple(nil)},
		{name: "Empty", want: "", in: configmap.Simple{}},
		{name: "Basic", want: "config1='one'", in: configmap.Simple{
			"config1": "one",
		}},
		{name: "Truthy", want: "config1='true',config2='true'", in: configmap.Simple{
			"config1": "true",
			"config2": "true",
		}},
		{name: "Quotable", want: `config1='"one"',config2=':two:',config3='''three''',config4='=four=',config5=',five,'`, in: configmap.Simple{
			"config1": `"one"`,
			"config2": `:two:`,
			"config3": `'three'`,
			"config4": `=four=`,
			"config5": `,five,`,
		}},
		{name: "Order", want: "config1='one',config2='two',config3='three',config4='four',config5='five'", in: configmap.Simple{
			"config5": "five",
			"config4": "four",
			"config3": "three",
			"config2": "two",
			"config1": "one",
		}},
		{name: "Escaping", want: "apple='',config1='o''n''e'", in: configmap.Simple{
			"config1": "o'n'e",
			"apple":   "",
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Check forwards
			params := tt.in.String()
			assert.Equal(t, tt.want, params)

			// Check config round trips through config parser
			remote := ":local," + params + ":"
			if params == "" {
				remote = ":local:"
			}
			what := fmt.Sprintf("remote = %q", remote)
			parsed, err := fspath.Parse(remote)
			require.NoError(t, err, what)
			if len(parsed.Config) != 0 || len(tt.in) != 0 {
				assert.Equal(t, tt.in, parsed.Config, what)
			}
		})
	}

}

func TestSimpleHuman(t *testing.T) {
	for _, tt := range []struct {
		name string
		want string
		in   configmap.Simple
	}{
		{name: "Nil", want: "", in: configmap.Simple(nil)},
		{name: "Empty", want: "", in: configmap.Simple{}},
		{name: "Basic", want: "config1=one", in: configmap.Simple{
			"config1": "one",
		}},
		{name: "Truthy", want: "config1,config2", in: configmap.Simple{
			"config1": "true",
			"config2": "true",
		}},
		{name: "Quotable", want: `config1='"one"',config2=':two:',config3='''three''',config4='=four=',config5=',five,'`, in: configmap.Simple{
			"config1": `"one"`,
			"config2": `:two:`,
			"config3": `'three'`,
			"config4": `=four=`,
			"config5": `,five,`,
		}},
		{name: "Order", want: "config1=one,config2=two,config3=three,config4=four,config5=five", in: configmap.Simple{
			"config5": "five",
			"config4": "four",
			"config3": "three",
			"config2": "two",
			"config1": "one",
		}},
		{name: "Escaping", want: "apple=,config1='o''n''e'", in: configmap.Simple{
			"config1": "o'n'e",
			"apple":   "",
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Check forwards
			params := tt.in.Human()
			assert.Equal(t, tt.want, params)

			// Check config round trips through config parser
			remote := ":local," + params + ":"
			if params == "" {
				remote = ":local:"
			}
			what := fmt.Sprintf("remote = %q", remote)
			parsed, err := fspath.Parse(remote)
			require.NoError(t, err, what)
			if len(parsed.Config) != 0 || len(tt.in) != 0 {
				assert.Equal(t, tt.in, parsed.Config, what)
			}
		})
	}

}
