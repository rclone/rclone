# Request Based SFTP API

The request based API allows for custom backends in a way similar to the http
package. In order to create a backend you need to implement 4 handler
interfaces; one for reading, one for writing, one for misc commands and one for
listing files. Each has 1 required method and in each case those methods take
the Request as the only parameter and they each return something different.
These 4 interfaces are enough to handle all the SFTP traffic in a simplified
manner.

The Request structure has 5 public fields which you will deal with.

- Method (string) - string name of incoming call
- Filepath (string) - path of file to act on
- Attrs ([]byte) - byte string of file attribute data
- Target (string) - target path for renames and sym-links

Below are the methods and a brief description of what they need to do.

### Fileread(*Request) (io.Reader, error)

Handler for "Get" method and returns an io.Reader for the file which the server
then sends to the client.

### Filewrite(*Request) (io.Writer, error)

Handler for "Put" method and returns an io.Writer for the file which the server
then writes the uploaded file to.

###    Filecmd(*Request) error

Handles "SetStat", "Rename", "Rmdir", "Mkdir"  and "Symlink" methods. Makes the
appropriate changes and returns nil for success or an filesystem like error
(eg. os.ErrNotExist).

### Fileinfo(*Request) ([]os.FileInfo, error)

Handles "List", "Stat", "Readlink" methods. Gathers/creates FileInfo structs
with the data on the files and returns in a list (list of 1 for Stat and
Readlink).


## TODO

- Add support for API users to see trace/debugging info of what is going on
inside SFTP server.
- Consider adding support for SFTP file append only mode.

