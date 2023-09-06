/*
Package swift provides an easy to use interface to Swift / Openstack Object Storage / Rackspace Cloud Files

# Standard Usage

Most of the work is done through the Container*() and Object*() methods.

All methods are safe to use concurrently in multiple go routines.

# Object Versioning

As defined by http://docs.openstack.org/api/openstack-object-storage/1.0/content/Object_Versioning-e1e3230.html#d6e983 one can create a container which allows for version control of files.  The suggested method is to create a version container for holding all non-current files, and a current container for holding the latest version that the file points to.  The container and objects inside it can be used in the standard manner, however, pushing a file multiple times will result in it being copied to the version container and the new file put in it's place.  If the current file is deleted, the previous file in the version container will replace it.  This means that if a file is updated 5 times, it must be deleted 5 times to be completely removed from the system.

# Rackspace Sub Module

This module specifically allows the enabling/disabling of Rackspace Cloud File CDN management on a container.  This is specific to the Rackspace API and not Swift/Openstack, therefore it has been placed in a submodule.  One can easily create a RsConnection and use it like the standard Connection to access and manipulate containers and objects.
*/
package swift
