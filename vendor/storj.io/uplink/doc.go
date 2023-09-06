// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package uplink is the main entrypoint to interacting with Storj Labs' decentralized
storage network.

Sign up for an account on a Satellite today! https://storj.io/

# Access Grants

The fundamental unit of access in the Storj Labs storage network is the Access Grant.
An access grant is a serialized structure that is internally comprised of an API Key,
a set of encryption key information, and information about which Storj Labs or
Tardigrade network Satellite is responsible for the metadata. An access grant is
always associated with exactly one Project on one Satellite.

If you don't already have an access grant, you will need make an account on a
Satellite, generate an API Key, and encapsulate that API Key with encryption
information into an access grant.

If you don't already have an account on a Satellite, first make one at
https://storj.io/ and note the Satellite you choose (such as
us1.storj.io, eu1.storj.io, etc). Then, make an API Key in the web interface.

The first step to any project is to generate a restricted access grant with the
minimal permissions that are needed. Access grants contains all encryption information
and they should be restricted as much as possible.

To make an access grant, you can create one using our Uplink CLI tool's 'share'
subcommand (after setting up the Uplink CLI tool), or you can make one as follows:

	access, err := uplink.RequestAccessWithPassphrase(ctx, satelliteAddress, apiKey, rootPassphrase)
	if err != nil {
	    return err
	}

	// create an access grant for reading bucket "logs"
	permission := uplink.ReadOnlyPermission()
	shared := uplink.SharePrefix{Bucket: "logs"}
	restrictedAccess, err := access.Share(permission, shared)
	if err != nil {
	    return err
	}

	// serialize the restricted access grant
	serializedAccess, err := restrictedAccess.Serialize()
	if err != nil {
	    return err
	}

In the above example, 'serializedAccess' is a human-readable string that represents
read-only access to just the "logs" bucket, and is only able to decrypt that one
bucket thanks to hierarchical deterministic key derivation.

Note: RequestAccessWithPassphrase is CPU-intensive, and your application's normal
lifecycle should avoid it and use ParseAccess where possible instead.

To revoke an access grant see the Project.RevokeAccess method.

# Multitenancy in a Single Application Bucket

A common architecture for building applications is to have a single bucket for the
entire application to store the objects of all users. In such architecture, it is
of utmost importance to guarantee that users can access only their objects but not
the objects of other users.

This can be achieved by implementing an app-specific authentication service that
generates an access grant for each user by restricting the main access grant of the
application. This user-specific access grant is restricted to access the objects
only within a specific key prefix defined for the user.

When initialized, the authentication server creates the main application access
grant with an empty passphrase as follows.

	appAccess, err := uplink.RequestAccessWithPassphrase(ctx, satellite, appAPIKey, "")

The authentication service does not hold any encryption information about users, so
the passphrase used to request the main application access grant does not matter.
The encryption keys related to user objects will be overridden in a next step on
the client-side. It is important that once set to a specific value, this passphrase
never changes in the future. Therefore, the best practice is to use an empty
passphrase.

Whenever a user is authenticated, the authentication service generates the
user-specific access grant as follows:

	// create a user access grant for accessing their files, limited for the next 8 hours
	now := time.Now()
	permission := uplink.FullPermission()
	// 2 minutes leeway to avoid time sync issues with the satellite
	permission.NotBefore = now.Add(-2 * time.Minute)
	permission.NotAfter = now.Add(8 * time.Hour)
	userPrefix := uplink.SharePrefix{
	    Bucket: appBucket,
	    Prefix: userID + "/",
	}
	userAccess, err := appAccess.Share(permission, userPrefix)
	if err != nil {
	    return err
	}

	// serialize the user access grant
	serializedAccess, err := userAccess.Serialize()
	if err != nil {
	    return err
	}

The userID is something that uniquely identifies the users in the application
and must never change.

Along with the user access grant, the authentication service should return a
user-specific salt. The salt must be always the same for this user. The salt size
is 16-byte or 32-byte.

Once the application receives the user-specific access grant and the user-specific
salt from the authentication service, it has to override the encryption key in the
access grant, so users can encrypt and decrypt their files with encryption keys
derived from their passphrase.

	userAccess, err = uplink.ParseAccess(serializedUserAccess)
	if err != nil {
	    return nil, err
	}

	saltedUserKey, err := uplink.DeriveEncryptionKey(userPassphrase, userSalt)
	if err != nil {
	    return nil, err
	}

	err = userAccess.OverrideEncryptionKey(appBucket, userID+"/", saltedUserKey)
	if err != nil {
	    return nil, err
	}

The user-specific access grant is now ready to use by the application.

# Projects

Once you have a valid access grant, you can open a Project with the access that
access grant allows for.

	project, err := uplink.OpenProject(ctx, access)
	if err != nil {
	    return err
	}
	defer project.Close()

Projects allow you to manage buckets and objects within buckets.

# Buckets

A bucket represents a collection of objects. You can upload, download, list, and delete objects of
any size or shape. Objects within buckets are represented by keys, where keys can optionally be
listed using the "/" delimiter.

Note: Objects and object keys within buckets are end-to-end encrypted, but bucket names
themselves are not encrypted, so the billing interface on the Satellite can show you bucket line
items.

	buckets := project.ListBuckets(ctx, nil)
	for buckets.Next() {
	    fmt.Println(buckets.Item().Name)
	}
	if err := buckets.Err(); err != nil {
	    return err
	}

# Download Object

Objects support a couple kilobytes of arbitrary key/value metadata, and arbitrary-size primary
data streams with the ability to read at arbitrary offsets.

	object, err := project.DownloadObject(ctx, "logs", "2020-04-18/webserver.log", nil)
	if err != nil {
	    return err
	}
	defer object.Close()

	_, err = io.Copy(w, object)
	return err

If you want to access only a small subrange of the data you uploaded, you can use
`uplink.DownloadOptions` to specify the download range.

	object, err := project.DownloadObject(ctx, "logs", "2020-04-18/webserver.log",
	    &uplink.DownloadOptions{Offset: 10, Length: 100})
	if err != nil {
	    return err
	}
	defer object.Close()

	_, err = io.Copy(w, object)
	return err

# List Objects

Listing objects returns an iterator that allows to walk through all the items:

	objects := project.ListObjects(ctx, "logs", nil)
	for objects.Next() {
	    item := objects.Item()
	    fmt.Println(item.IsPrefix, item.Key)
	}
	if err := objects.Err(); err != nil {
	    return err
	}
*/
package uplink
