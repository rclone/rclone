package ftpopt

// Help contains text describing the http server to add to the command
// help.
var Help = `
### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

#### Authentication

By default this will serve files without needing a login.

You can set a single username and password with the --user and --pass flags.
`

// Options contains options for the http Server
type Options struct {
	//TODO add more options
	ListenAddr   string // Port to listen on
	PassivePorts string // Passive ports range
	BasicUser    string // single username for basic auth if not using Htpasswd
	BasicPass    string // password for BasicUser
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	ListenAddr:   "localhost:2121",
	PassivePorts: "30000-32000",
	BasicUser:    "anonymous",
	BasicPass:    "",
}
