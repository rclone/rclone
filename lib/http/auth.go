package http

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// AuthHelp returns text describing the http authentication to add to the command help.
func AuthHelp(prefix string) string {
	help := `#### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the ` + "`--{{ .Prefix }}user` and `--{{ .Prefix }}pass`" + ` flags.

If no static users are configured by either of the above methods, and client
certificates are required by the ` + "`--client-ca`" + ` flag passed to the server, the
client certificate common name will be considered as the username.

Use ` + "`--{{ .Prefix }}htpasswd /path/to/htpasswd`" + ` to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use ` + "`--{{ .Prefix }}realm`" + ` to set the authentication realm.

Use ` + "`--{{ .Prefix }}salt`" + ` to change the password hashing salt from the default.

`
	tmpl, err := template.New("auth help").Parse(help)
	if err != nil {
		fs.Fatal(nil, fmt.Sprint("Fatal error parsing template", err))
	}

	data := struct {
		Prefix string
	}{
		Prefix: prefix,
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		fs.Fatal(nil, fmt.Sprint("Fatal error executing template", err))
	}
	return buf.String()
}

// CustomAuthFn if used will be used to authenticate user, pass. If an error
// is returned then the user is not authenticated.
//
// If a non nil value is returned then it is added to the context under the key
type CustomAuthFn func(user, pass string) (value interface{}, err error)

// AuthConfigInfo descripts the Options in use
var AuthConfigInfo = fs.Options{{
	Name:    "htpasswd",
	Default: "",
	Help:    "A htpasswd file - if not provided no authentication is done",
}, {
	Name:    "realm",
	Default: "",
	Help:    "Realm for authentication",
}, {
	Name:    "user",
	Default: "",
	Help:    "User name for authentication",
}, {
	Name:    "pass",
	Default: "",
	Help:    "Password for authentication",
}, {
	Name:    "salt",
	Default: "dlPL2MqE",
	Help:    "Password hashing salt",
}}

// AuthConfig contains options for the http authentication
type AuthConfig struct {
	HtPasswd     string       `config:"htpasswd"`   // htpasswd file - if not provided no authentication is done
	Realm        string       `config:"realm"`      // realm for authentication
	BasicUser    string       `config:"user"`       // single username for basic auth if not using Htpasswd
	BasicPass    string       `config:"pass"`       // password for BasicUser
	Salt         string       `config:"salt"`       // password hashing salt
	CustomAuthFn CustomAuthFn `json:"-" config:"-"` // custom Auth (not set by command line flags)
}

// AddFlagsPrefix adds flags to the flag set for AuthConfig
func (cfg *AuthConfig) AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string) {
	flags.StringVarP(flagSet, &cfg.HtPasswd, prefix+"htpasswd", "", cfg.HtPasswd, "A htpasswd file - if not provided no authentication is done", prefix)
	flags.StringVarP(flagSet, &cfg.Realm, prefix+"realm", "", cfg.Realm, "Realm for authentication", prefix)
	flags.StringVarP(flagSet, &cfg.BasicUser, prefix+"user", "", cfg.BasicUser, "User name for authentication", prefix)
	flags.StringVarP(flagSet, &cfg.BasicPass, prefix+"pass", "", cfg.BasicPass, "Password for authentication", prefix)
	flags.StringVarP(flagSet, &cfg.Salt, prefix+"salt", "", cfg.Salt, "Password hashing salt", prefix)
}

// AddAuthFlagsPrefix adds flags to the flag set for AuthConfig
func AddAuthFlagsPrefix(flagSet *pflag.FlagSet, prefix string, cfg *AuthConfig) {
	cfg.AddFlagsPrefix(flagSet, prefix)
}

// DefaultAuthCfg returns a new config which can be customized by command line flags
//
// Note that this needs to be kept in sync with AuthConfigInfo above and
// can be removed when all callers have been converted.
func DefaultAuthCfg() AuthConfig {
	return AuthConfig{
		Salt: "dlPL2MqE",
	}
}
