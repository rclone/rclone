package auth

import (
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/lib/http"
	"github.com/spf13/pflag"
)

// Help contains text describing the http authentication to add to the command
// help.
var Help = `
#### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the --user and --pass flags.

Use --htpasswd /path/to/htpasswd to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use --realm to set the authentication realm.
`

// CustomAuthFn if used will be used to authenticate user, pass. If an error
// is returned then the user is not authenticated.
//
// If a non nil value is returned then it is added to the context under the key
type CustomAuthFn func(user, pass string) (value interface{}, err error)

// Options contains options for the http authentication
type Options struct {
	HtPasswd  string       // htpasswd file - if not provided no authentication is done
	Realm     string       // realm for authentication
	BasicUser string       // single username for basic auth if not using Htpasswd
	BasicPass string       // password for BasicUser
	Auth      CustomAuthFn `json:"-"` // custom Auth (not set by command line flags)
}

// Auth instantiates middleware that authenticates users based on the configuration
func Auth(opt Options) http.Middleware {
	if opt.Auth != nil {
		return CustomAuth(opt.Auth, opt.Realm)
	} else if opt.HtPasswd != "" {
		return HtPasswdAuth(opt.HtPasswd, opt.Realm)
	} else if opt.BasicUser != "" {
		return SingleAuth(opt.BasicUser, opt.BasicPass, opt.Realm)
	}
	return nil
}

// Options set by command line flags
var (
	Opt = Options{}
)

// AddFlagsPrefix adds flags for http/auth
func AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string, Opt *Options) {
	flags.StringVarP(flagSet, &Opt.HtPasswd, prefix+"htpasswd", "", Opt.HtPasswd, "htpasswd file - if not provided no authentication is done")
	flags.StringVarP(flagSet, &Opt.Realm, prefix+"realm", "", Opt.Realm, "realm for authentication")
	flags.StringVarP(flagSet, &Opt.BasicUser, prefix+"user", "", Opt.BasicUser, "User name for authentication.")
	flags.StringVarP(flagSet, &Opt.BasicPass, prefix+"pass", "", Opt.BasicPass, "Password for authentication.")
}

// AddFlags adds flags for the http/auth
func AddFlags(flagSet *pflag.FlagSet) {
	AddFlagsPrefix(flagSet, "", &Opt)
}
