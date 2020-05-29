// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/*
Package uplink is the main entrypoint to interacting with Storj Labs' decentralized
storage network.

Sign up for an account on a Satellite today! https://tardigrade.io/satellites/

Access Grants

The fundamental unit of access in the Storj Labs storage network is the Access Grant.
An access grant is a serialized structure that is internally comprised of an API Key,
a set of encryption key information, and information about which Storj Labs or
Tardigrade network Satellite is responsible for the metadata. An access grant is
always associated with exactly one Project on one Satellite.

If you don't already have an access grant, you will need make an account on a
Satellite, generate an API Key, and encapsulate that API Key with encryption
information into an access grant.

If you don't already have an account on a Satellite, first make one at
https://tardigrade.io/satellites/ and note the Satellite you choose (such as
us-central-1.tardigrade.io, europe-west-1.tardigrade.io, etc). Then, make an
API Key in the web interface.

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
    permissions := uplink.ReadOnlyPermission()
    shared := uplink.SharePrefix{Bucket: "logs"}
    restrictedAccess, err := access.Share(permissions, shared)
    if err != nil {
        return err
    }

    // serialize the restricted access
    serializedAccess, err := restrictedAccess.Serialize()
    if err != nil {
        return err
    }

In the above example, 'serializedAccess' is a human-readable string that represents
read-only access to just the "logs" bucket, and is only able to decrypt that one
bucket thanks to hierarchical deterministic key derivation.

Note: RequestAccessWithPassphrase is CPU-intensive, and your application's normal
lifecycle should avoid it and use ParseAccess where possible instead.

Projects

Once you have a valid access grant, you can open a Project with the access that
access grant allows for.

    project, err := uplink.OpenProject(ctx, access)
    if err != nil {
        return err
    }
    defer project.Close()


Projects allow you to manage buckets and objects within buckets.

Buckets

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

Download Object

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

List Objects

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
