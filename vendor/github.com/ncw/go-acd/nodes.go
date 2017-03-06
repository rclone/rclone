// Copyright (c) 2015 Serge Gebhardt. All rights reserved.
//
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package acd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"

	"github.com/google/go-querystring/query"
)

var (
	// ErrorNodeNotFound is returned from GetFile, GetFolder, GetNode
	ErrorNodeNotFound = errors.New("Node not found")
)

// NodesService provides access to the nodes in the Amazon Cloud Drive API.
//
// See: https://developer.amazon.com/public/apis/experience/cloud-drive/content/nodes
type NodesService struct {
	client *Client
}

// GetRoot gets the root folder of the Amazon Cloud Drive.
func (s *NodesService) GetRoot() (*Folder, *http.Response, error) {
	opts := &NodeListOptions{Filters: "kind:FOLDER AND isRoot:true"}

	roots, resp, err := s.GetNodes(opts)
	if err != nil {
		return nil, resp, err
	}

	if len(roots) < 1 {
		return nil, resp, errors.New("No root found")
	}

	return &Folder{roots[0]}, resp, nil
}

// GetAllNodes gets the list of all nodes.
func (s *NodesService) GetAllNodes(opts *NodeListOptions) ([]*Node, *http.Response, error) {
	return s.listAllNodes("nodes", opts)
}

// GetNodes gets a list of nodes, up until the limit (either default or the one set in opts).
func (s *NodesService) GetNodes(opts *NodeListOptions) ([]*Node, *http.Response, error) {
	nodes, res, err := s.listNodes("nodes", opts)
	return nodes, res, err
}

func (s *NodesService) listAllNodes(url string, opts *NodeListOptions) ([]*Node, *http.Response, error) {
	// Need opts to maintain state (NodeListOptions.reachedEnd)
	if opts == nil {
		opts = &NodeListOptions{}
	}

	result := make([]*Node, 0, 200)

	for {
		nodes, resp, err := s.listNodes(url, opts)
		if err != nil {
			return result, resp, err
		}
		if nodes == nil {
			break
		}

		result = append(result, nodes...)
	}

	return result, nil, nil
}

func (s *NodesService) listNodes(url string, opts *NodeListOptions) ([]*Node, *http.Response, error) {
	if opts != nil && opts.reachedEnd {
		return nil, nil, nil
	}

	url, err := addOptions(url, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewMetadataRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	nodeList := &nodeListInternal{}
	resp, err := s.client.Do(req, nodeList)
	if err != nil {
		return nil, resp, err
	}

	if opts != nil {
		if nodeList.NextToken != nil {
			opts.StartToken = *nodeList.NextToken
		} else {
			opts.reachedEnd = true
		}
	}

	nodes := nodeList.Data
	for _, node := range nodes {
		node.service = s
	}

	return nodes, resp, nil
}

type nodeListInternal struct {
	Count     *uint64 `json:"count"`
	NextToken *string `json:"nextToken"`
	Data      []*Node `json:"data"`
}

// Node represents a digital asset on the Amazon Cloud Drive, including files
// and folders, in a parent-child relationship. A node contains only metadata
// (e.g. folder) or it contains metadata and content (e.g. file).
type Node struct {
	Id                *string  `json:"id"`
	Name              *string  `json:"name"`
	Kind              *string  `json:"kind"`
	ModifiedDate      *string  `json:"modifiedDate"`
	Parents           []string `json:"parents"`
	Status            *string  `json:"status"`
	ContentProperties *struct {
		Size        *uint64 `json:"size"`
		Md5         *string `json:"md5"`
		ContentType *string `json:"contentType"`
	} `json:"contentProperties"`
	TempURL string `json:"tempLink"`

	service *NodesService
}

// NodeFromId constructs a skeleton Node from an Id and a NodeService
func NodeFromId(ID string, service *NodesService) *Node {
	return &Node{
		Id:      &ID,
		service: service,
	}
}

// IsFile returns whether the node represents a file.
func (n *Node) IsFile() bool {
	return n.Kind != nil && *n.Kind == "FILE"
}

// IsFolder returns whether the node represents a folder.
func (n *Node) IsFolder() bool {
	return n.Kind != nil && *n.Kind == "FOLDER"
}

// Typed returns the Node typed as either File or Folder.
func (n *Node) Typed() interface{} {
	if n.IsFile() {
		return &File{n}
	}

	if n.IsFolder() {
		return &Folder{n}
	}

	return n
}

// GetTempURL sets the TempURL for the node passed in if it isn't already set
func (n *Node) GetTempURL() (*http.Response, error) {
	if n.TempURL != "" {
		return nil, nil
	}
	url := fmt.Sprintf("nodes/%s?tempLink=true", *n.Id)
	req, err := n.service.client.NewMetadataRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	node := &Node{}
	resp, err := n.service.client.Do(req, node)
	if err != nil {
		return resp, err
	}

	if node.TempURL == "" {
		return resp, fmt.Errorf("Couldn't read TempURL")
	}

	// Set the TempURL in the node
	n.TempURL = node.TempURL
	return resp, nil
}

// GetMetadata return a pretty-printed JSON string of the node's metadata
func (n *Node) GetMetadata() (string, error) {
	url := fmt.Sprintf("nodes/%s?tempLink=true", *n.Id)
	req, err := n.service.client.NewMetadataRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	_, err = n.service.client.Do(req, buf)
	if err != nil {
		return "", err
	}

	md := &bytes.Buffer{}
	err = json.Indent(md, buf.Bytes(), "", "    ")
	if err != nil {
		return "", err
	}

	return md.String(), nil
}

type replaceParent struct {
	FromParent string `json:"fromParent"`
	ChildID    string `json:"childId"`
}

// ReplaceParent puts Node n below a new parent while removing the old one at the same time.
// This is equivalent to calling AddParent and RemoveParent sequentially, but
// only needs one REST call. Can return a 409 Conflict if there's already a
// file or folder in the new location with the same name as Node n.
func (n *Node) ReplaceParent(oldParentID string, newParentID string) (*http.Response, error) {
	body := &replaceParent{
		FromParent: oldParentID,
		ChildID:    *n.Id,
	}
	url := fmt.Sprintf("nodes/%s/children", newParentID)
	req, err := n.service.client.NewMetadataRequest("POST", url, &body)
	if err != nil {
		return nil, err
	}
	resp, err := n.service.client.Do(req, nil)
	if err != nil {
		return resp, err
	}
	n.Parents = []string{newParentID}

	err = resp.Body.Close()
	return resp, err
}

// AddParent adds an additional parent to Node n. Can return a 409 Conflict if there's
// already a file or folder below the new parent with the same name as Node n.
func (n *Node) AddParent(newParentID string) (*http.Response, error) {
	return n.changeParents(newParentID, true)
}

// RemoveParent removes a parent from Node n. If all parents are removed, the file is instead
// attached to the absolute root folder of AmazonDrive.
func (n *Node) RemoveParent(parentID string) (*http.Response, error) {
	return n.changeParents(parentID, false)
}

func (n *Node) changeParents(parentID string, add bool) (*http.Response, error) {
	method := "DELETE"
	if add {
		method = "PUT"
	}

	url := fmt.Sprintf("nodes/%s/children/%s", parentID, *n.Id)
	req, err := n.service.client.NewMetadataRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := n.service.client.Do(req, nil)
	if err != nil {
		return resp, err
	}
	if add {
		n.Parents = append(n.Parents, parentID)
	} else {
		var removeIndex int
		for i := 0; i < len(n.Parents); i++ {
			if n.Parents[i] == parentID {
				removeIndex = i
				break
			}
		}
		n.Parents = append(n.Parents[:removeIndex], n.Parents[removeIndex+1:]...)
	}

	err = resp.Body.Close()
	return resp, err
}

// renameNode is a cut down set of parameters for renaming nodes
type renameNode struct {
	Name string `json:"name"`
}

// Rename node
func (n *Node) Rename(newName string) (*Node, *http.Response, error) {
	url := fmt.Sprintf("nodes/%s", *n.Id)
	metadata := renameNode{
		Name: newName,
	}

	node := &Node{service: n.service}
	req, err := n.service.client.NewMetadataRequest("PATCH", url, &metadata)
	if err != nil {
		return nil, nil, err
	}

	resp, err := n.service.client.Do(req, node)
	return node, resp, err
}

// Trash places Node n into the trash.  If the node is a directory it
// places it and all of its contents into the trash.
func (n *Node) Trash() (*http.Response, error) {
	url := fmt.Sprintf("trash/%s", *n.Id)
	req, err := n.service.client.NewMetadataRequest("PUT", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := n.service.client.Do(req, nil)
	if err != nil {
		return resp, err
	}
	err = resp.Body.Close()
	if err != nil {
		return resp, err
	}
	return resp, nil

}

// Restore moves a previously trashed Node n back into all its connected parents
func (n *Node) Restore() (*Node, *http.Response, error) {
	url := fmt.Sprintf("trash/%s/restore", *n.Id)
	req, err := n.service.client.NewMetadataRequest("POST", url, nil)
	if err != nil {
		return nil, nil, err
	}
	node := &Node{service: n.service}
	resp, err := n.service.client.Do(req, node)
	if err != nil {
		return nil, resp, err
	}
	err = resp.Body.Close()
	return node, resp, err
}

// File represents a file on the Amazon Cloud Drive.
type File struct {
	*Node
}

// OpenHeaders opens the content of the file f for read
//
// Extra headers for the GET can be passed in in headers
//
// You must call in.Close() when finished
func (f *File) OpenHeaders(headers map[string]string) (in io.ReadCloser, resp *http.Response, err error) {
	url := fmt.Sprintf("nodes/%s/content", *f.Id)
	req, err := f.service.client.NewContentRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	resp, err = f.service.client.Do(req, nil)
	if err != nil {
		return nil, resp, err
	}
	return resp.Body, resp, nil
}

// Open the content of the file f for read
//
// You must call in.Close() when finished
func (f *File) Open() (in io.ReadCloser, resp *http.Response, err error) {
	return f.OpenHeaders(nil)
}

// OpenTempURLHeaders opens the content of the file f for read from the TempURL
//
// Pass in an http Client (without authorization) for the download.
//
// You must call in.Close() when finished
func (f *File) OpenTempURLHeaders(client *http.Client, headers map[string]string) (in io.ReadCloser, resp *http.Response, err error) {
	resp, err = f.GetTempURL()
	if err != nil {
		return nil, resp, err
	}
	req, err := http.NewRequest("GET", f.TempURL, nil)
	if err != nil {
		return nil, nil, err
	}
	if f.service.client.UserAgent != "" {
		req.Header.Add("User-Agent", f.service.client.UserAgent)
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}
	resp, err = client.Do(req)
	if err != nil {
		return nil, resp, err
	}
	return resp.Body, resp, nil
}

// OpenTempURL opens the content of the file f for read from the TempURL
//
// Pass in an http Client (without authorization) for the download.
//
// You must call in.Close() when finished
func (f *File) OpenTempURL(client *http.Client) (in io.ReadCloser, resp *http.Response, err error) {
	return f.OpenTempURLHeaders(client, nil)
}

// Download fetches the content of file f and stores it into the file pointed
// to by path. Errors if the file at path already exists. Does not create the
// intermediate directories in path.
func (f *File) Download(path string) (*http.Response, error) {
	url := fmt.Sprintf("nodes/%s/content", *f.Id)
	req, err := f.service.client.NewContentRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	resp, err := f.service.client.Do(req, out)
	return resp, err
}

// Folder represents a folder on the Amazon Cloud Drive.
type Folder struct {
	*Node
}

// FolderFromId constructs a skeleton Folder from an Id and a NodeService
func FolderFromId(ID string, service *NodesService) *Folder {
	return &Folder{
		Node: NodeFromId(ID, service),
	}
}

// GetAllChildren gets the list of all children.
func (f *Folder) GetAllChildren(opts *NodeListOptions) ([]*Node, *http.Response, error) {
	url := fmt.Sprintf("nodes/%s/children", *f.Id)
	return f.service.listAllNodes(url, opts)
}

// GetChildren gets a list of children, up until the limit (either
// default or the one set in opts).
func (f *Folder) GetChildren(opts *NodeListOptions) ([]*Node, *http.Response, error) {
	url := fmt.Sprintf("nodes/%s/children", *f.Id)
	return f.service.listNodes(url, opts)
}

// GetFolder gets the subfolder by name. It is an error if not exactly
// one subfolder is found.
//
// If it isn't found then it returns the error ErrorNodeNotFound
func (f *Folder) GetFolder(name string) (*Folder, *http.Response, error) {
	n, resp, err := f.GetNode(name)
	if err != nil {
		return nil, resp, err
	}

	res, ok := n.Typed().(*Folder)
	if !ok {
		err := fmt.Errorf("Node '%s' is not a folder", name)
		return nil, resp, err
	}

	return res, resp, nil
}

// createNode is a cut down set of parameters for creating nodes
type createNode struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Parents []string `json:"parents"`
}

// CreateFolder makes a new folder with the given name.
//
// The new Folder is returned
func (f *Folder) CreateFolder(name string) (*Folder, *http.Response, error) {
	createFolder := createNode{
		Name:    name,
		Kind:    "FOLDER",
		Parents: []string{*f.Id},
	}
	req, err := f.service.client.NewMetadataRequest("POST", "nodes", &createFolder)
	if err != nil {
		return nil, nil, err
	}

	folder := &Folder{&Node{service: f.service}}
	resp, err := f.service.client.Do(req, folder)
	if err != nil {
		return nil, resp, err
	}
	return folder, resp, nil

}

// GetFile gets the file by name. It is an error if not exactly one file is found.
//
// If it isn't found then it returns the error ErrorNodeNotFound
func (f *Folder) GetFile(name string) (*File, *http.Response, error) {
	n, resp, err := f.GetNode(name)
	if err != nil {
		return nil, resp, err
	}

	res, ok := n.Typed().(*File)
	if !ok {
		err := fmt.Errorf("Node '%s' is not a file", name)
		return nil, resp, err
	}

	return res, resp, nil
}

var escapeForFilterRe = regexp.MustCompile(`([+\-&|!(){}\[\]^'"~*?:\\ ])`)

// EscapeForFilter escapes an abitrary string for use as a filter
// query parameter.
//
// Special characters that are part of the query syntax will be
// escaped. The list of special characters are:
//
// + - & | ! ( ) { } [ ] ^ ' " ~ * ? : \
//
// Additionally, space will be escaped. Characters are escaped by
// using \ before the character.
func EscapeForFilter(s string) string {
	return escapeForFilterRe.ReplaceAllString(s, `\$1`)
}

// GetNode gets the node by name. It is an error if not exactly one node is found.
//
// If it isn't found then it returns the error ErrorNodeNotFound
func (f *Folder) GetNode(name string) (*Node, *http.Response, error) {
	filter := fmt.Sprintf(`parents:"%v" AND name:"%s"`, *f.Id, EscapeForFilter(name))
	opts := &NodeListOptions{Filters: filter}

	nodes, resp, err := f.service.GetNodes(opts)
	if err != nil {
		return nil, resp, err
	}

	if len(nodes) < 1 {
		return nil, resp, ErrorNodeNotFound
	}
	if len(nodes) > 1 {
		err := fmt.Errorf("Too many nodes '%s' found (%v)", name, len(nodes))
		return nil, resp, err
	}

	return nodes[0], resp, nil
}

// WalkNodes walks the given node hierarchy, getting each node along the way, and returns
// the deepest node. If an error occurs, returns the furthest successful node and the list
// of HTTP responses.
func (f *Folder) WalkNodes(names ...string) (*Node, []*http.Response, error) {
	resps := make([]*http.Response, 0, len(names))

	if len(names) == 0 {
		return f.Node, resps, nil
	}

	// process each node except the last one
	fp := f
	for _, name := range names[:len(names)-1] {
		fn, resp, err := fp.GetFolder(name)
		resps = append(resps, resp)
		if err != nil {
			return fp.Node, resps, err
		}

		fp = fn
	}

	// process the last node
	nl, resp, err := fp.GetNode(names[len(names)-1])
	resps = append(resps, resp)
	if err != nil {
		return fp.Node, resps, err
	}

	return nl, resps, nil
}

// Put stores the data read from in at path as name on the Amazon Cloud Drive.
// Errors if the file already exists on the drive.
func (s *NodesService) putOrOverwrite(in io.Reader, httpVerb, url, name, metadata string) (*File, *http.Response, error) {
	var bodyReader io.Reader

	bodyReader, bodyWriter := io.Pipe()
	writer := multipart.NewWriter(bodyWriter)
	contentType := writer.FormDataContentType()
	contentLength := int64(-1)

	buf := make([]byte, 1)
	n, err := io.ReadFull(in, buf)
	isZeroLength := err == io.EOF
	if !isZeroLength && err != nil {
		return nil, nil, err
	}
	in = io.MultiReader(bytes.NewReader(buf[:n]), in)

	errChan := make(chan error, 1)
	go func() {
		defer bodyWriter.Close()
		var err error

		if metadata != "" {
			err = writer.WriteField("metadata", string(metadata))
			if err != nil {
				errChan <- err
				return
			}
		}

		part, err := writer.CreateFormFile("content", name)
		if err != nil {
			errChan <- err
			return
		}
		if _, err := io.Copy(part, in); err != nil {
			errChan <- err
			return
		}
		errChan <- writer.Close()
	}()

	if isZeroLength {
		buf, err := ioutil.ReadAll(bodyReader)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = bytes.NewReader(buf)
		contentLength = int64(len(buf))
	}

	req, err := s.client.NewContentRequest(httpVerb, url, bodyReader)
	if err != nil {
		return nil, nil, err
	}

	req.ContentLength = contentLength
	req.Header.Add("Content-Type", contentType)

	file := &File{&Node{service: s}}
	resp, err := s.client.Do(req, file)
	if err != nil {
		return nil, resp, err
	}

	err = <-errChan
	if err != nil {
		return nil, resp, err
	}

	return file, resp, err
}

// Put stores the data read from in at path as name on the Amazon Cloud Drive.
// Errors if the file already exists on the drive.
func (f *Folder) Put(in io.Reader, name string) (*File, *http.Response, error) {
	metadata := createNode{
		Name:    name,
		Kind:    "FILE",
		Parents: []string{*f.Id},
	}
	metadataJSON, err := json.Marshal(&metadata)
	if err != nil {
		return nil, nil, err
	}
	return f.service.putOrOverwrite(in, "POST", "nodes?suppress=deduplication", name, string(metadataJSON))
}

// Overwrite updates the file contents from in
func (f *File) Overwrite(in io.Reader) (*File, *http.Response, error) {
	url := fmt.Sprintf("nodes/%s/content", *f.Id)
	return f.service.putOrOverwrite(in, "PUT", url, *f.Name, "")
}

// PutSized stores the data read from in at path as name on the Amazon
// Cloud Drive.  Errors if the file already exists on the drive.
//
// Deprecated: no longer needed - just use Put
func (f *Folder) PutSized(in io.Reader, _ int64, name string) (*File, *http.Response, error) {
	return f.Put(in, name)
}

// OverwriteSized updates the file contents from in
//
// Deprecated: no longer needed - just use Overwrite
func (f *File) OverwriteSized(in io.Reader, _ int64) (*File, *http.Response, error) {
	return f.Overwrite(in)
}

// Upload stores the content of file at path as name on the Amazon Cloud Drive.
// Errors if the file already exists on the drive.
func (f *Folder) Upload(path, name string) (*File, *http.Response, error) {
	in, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer in.Close()
	return f.Put(in, name)
}

// NodeListOptions holds the options when getting a list of nodes, such as the filter,
// sorting and pagination.
type NodeListOptions struct {
	Limit   uint   `url:"limit,omitempty"`
	Filters string `url:"filters,omitempty"`
	Sort    string `url:"sort,omitempty"`

	// Token where to start for next page (internal)
	StartToken string `url:"startToken,omitempty"`
	reachedEnd bool
}

// addOptions adds the parameters in opts as URL query parameters to s.  opts
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opts interface{}) (string, error) {
	v := reflect.ValueOf(opts)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opts)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}
