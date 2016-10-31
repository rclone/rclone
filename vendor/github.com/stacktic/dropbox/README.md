dropbox
=======
Go client library for the Dropbox core and Datastore API with support for uploading and downloading encrypted files.

Support of the Datastore API should be considered as a beta version.

Prerequisite
------------
To use this library, you must have a valid client ID (app key) and client secret (app secret) provided by Dropbox.<br>
To register a new client application, please visit https://www.dropbox.com/developers/apps/create

Installation
------------
This library depends on the oauth2 package, it can be installed with the go get command:

    $ go get golang.org/x/oauth2

This package can be installed with the go get command:

    $ go get github.com/stacktic/dropbox


Examples
--------
This simple 4-step example will show you how to create a folder:

    package main

    import (
        "dropbox"
        "fmt"
    )

    func main() {
        var err error
        var db *dropbox.Dropbox

        var clientid, clientsecret string
        var token string

        clientid = "xxxxx"
        clientsecret = "yyyyy"
        token = "zzzz"

        // 1. Create a new dropbox object.
        db = dropbox.NewDropbox()

        // 2. Provide your clientid and clientsecret (see prerequisite).
        db.SetAppInfo(clientid, clientsecret)

        // 3. Provide the user token.
        db.SetAccessToken(token)

        // 4. Send your commands.
        // In this example, you will create a new folder named "demo".
        folder := "demo"
        if _, err = db.CreateFolder(folder); err != nil {
            fmt.Printf("Error creating folder %s: %s\n", folder, err)
        } else {
            fmt.Printf("Folder %s successfully created\n", folder)
        }
    }

If you do not know the user token, you can replace step 3 by a call to the Auth method:

        // This method will ask the user to visit an URL and paste the generated code.
        if err = db.Auth(); err != nil {
            fmt.Println(err)
            return
        }
        // You can now retrieve the token if you want.
        token = db.AccessToken()

If you want a more complete example, please check the following project: https://github.com/stacktic/dbox.

Documentation
-------------

API documentation can be found here: http://godoc.org/github.com/stacktic/dropbox.
