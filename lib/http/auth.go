package http

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/pflag"
)

// AuthHelp contains text describing the http authentication to add to the command help.
var AuthHelp = `
#### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the ` + "`--user` and `--pass`" + ` flags.

Use ` + "`--htpasswd /path/to/htpasswd`" + ` to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use ` + "`--realm`" + ` to set the authentication realm.

Use ` + "`--salt`" + ` to change the password hashing salt from the default.
`

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
	flags.StringVarP(flagSet, &cfg.HtPasswd, prefix+"htpasswd", "", cfg.HtPasswd, "A htpasswd file - if not provided no authentication is done")
	flags.StringVarP(flagSet, &cfg.Realm, prefix+"realm", "", cfg.Realm, "Realm for authentication")
	flags.StringVarP(flagSet, &cfg.BasicUser, prefix+"user", "", cfg.BasicUser, "User name for authentication")
	flags.StringVarP(flagSet, &cfg.BasicPass, prefix+"pass", "", cfg.BasicPass, "Password for authentication")
	flags.StringVarP(flagSet, &cfg.Salt, prefix+"salt", "", cfg.Salt, "Password hashing salt")
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
