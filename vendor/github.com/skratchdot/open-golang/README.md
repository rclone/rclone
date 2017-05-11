# open

## Description ##

    Open a file, directory, or URI using the OS's default application for
    that object type. Optionally, you can specify an application to use.

    This is a proxy for the following commands:

	        OSX: "open"
	    Windows: "start"
	Linux/Other: "xdg-open"

    This is a golang port of the node.js module:
    https://github.com/pwnall/node-open


## Documentation ##

[http://godoc.org/github.com/skratchdot/open-golang/open](http://godoc.org/github.com/skratchdot/open-golang/open)

## Import ##

    import "github.com/skratchdot/open-golang/open"

## Usage ##

### open google.com in the user's default browser (method 1):

	open.Run("https://google.com/")
	
### open google.com in the user's default browser (method 2):

	open.Start("https://google.com")

### you can listen for errors

	err := open.Run("https://google.com/")
	
### you can specify the program to use

	open.RunWith("https://google.com/", "firefox")	


## Functions ##

### func Run(input string) error

    Open a file, directory, or URI using the OS's default application for
    that object type. Wait for the open command to complete.

### func RunWith(input string, appName string) error

    Open a file, directory, or URI using the specified application. Wait for
    the open command to complete.

### func Start(input string) error

    Open a file, directory, or URI using the OS's default application for
    that object type. Don't wait for the open command to complete.

### func StartWith(input string, appName string) error

    Open a file, directory, or URI using the specified application. Don't
    wait for the open command to complete.


## License ##

Copyright (c) 2013 skratchdot  
Licensed under the MIT license.
