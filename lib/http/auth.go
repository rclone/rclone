package http

import (
	"bytes"
	"html/template"
	"log"

	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// AuthHelp returns text describing the http authentication to add to the command help.
func AuthHelp(prefix string) string {
	help := `
#### Authentication

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
		log.Fatal("Fatal error parsing template", err)
	}

	data := struct {
		Prefix string
	}{
		Prefix: prefix,
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		log.Fatal("Fatal error executing template", err)
	}
	return buf.String()
}

// CustomAuthFn if used will be used to authenticate user, pass. If an error
// is returned then the user is not authenticated.
//
// If a non nil value is returned then it is added to the context under the key
type CustomAuthFn func(user, pass string) (value interface{}, err error)

// AuthConfig contains options for the http authentication
type AuthConfig struct {
	HtPasswd     string       // htpasswd file - if not provided no authentication is done
	Realm        string       // realm for authentication
	BasicUser    string       // single username for basic auth if not using Htpasswd
	BasicPass    string       // password for BasicUser
	Salt         string       // password hashing salt
	CustomAuthFn CustomAuthFn `json:"-"` // custom Auth (not set by command line flags)
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
func DefaultAuthCfg() AuthConfig {
	return AuthConfig{
		Salt: "dlPL2MqE",
	}
}
