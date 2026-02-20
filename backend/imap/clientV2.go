// Package imap implements a provider for imap servers.
package imap

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// utility functions

func reverse(input string) string {
	// Get Unicode code points.
	n := 0
	rune := make([]rune, len(input))
	for _, r := range input {
		rune[n] = r
		n++
	}
	rune = rune[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		rune[i], rune[n-1-i] = rune[n-1-i], rune[i]
	}
	// Convert back to UTF-8.
	return string(rune)
}

func topDir(input string) (string, string) {
	right, top := path.Split(reverse(input))
	return strings.Trim(reverse(top), "/"), strings.Trim(reverse(right), "/")
}

func getMatches(root string, list []string) []string {
	var value string
	// remove leading/trailing slashes from root
	root = strings.Trim(root, "/")
	// check list for matching prefix
	result := []string{}
	for _, element := range list {
		// remove leading/trailing slashes from element
		element = strings.Trim(element, "/")
		// check if element matches
		if root == "" {
			// looking in root, return top dir
			value, _ = topDir(element)
		} else if strings.HasPrefix(element, root+"/") {
			// check if element starts with root
			value, _ = topDir(strings.TrimPrefix(element, root+"/"))
		} else {
			// no match skip
			value = ""
		}
		// add if value not empty and not in result array
		if value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	//
	return result
}

// namespace

type namespace struct {
	nsType    string
	dir       string
	mailbox   string
	delimiter string
}

func (ns *namespace) mailboxToDir(mailbox string) string {
	return strings.ReplaceAll(mailbox, ns.delimiter, "/")
}

func (ns *namespace) dirToMailbox(dir string) string {
	return strings.ReplaceAll(dir, "/", ns.delimiter)
}

func (ns *namespace) matchesMailbox(mailbox string) bool {
	return strings.HasPrefix(mailbox, ns.mailbox)
}

func (ns *namespace) matchesDir(dir string) bool {
	return strings.HasPrefix(dir, ns.dir)
}

/*
func (ns *namespace) isPersonal() bool {
	return ns.nsType=="P"
}

func (ns *namespace) isShared() bool {
	return ns.nsType=="S"
}

func (ns *namespace) isOther() bool {
	return ns.nsType=="O"
}
*/

// mail client v2

type mailclient2 struct {
	conn       *imapclient.Client
	f          *Fs
	namespaces []*namespace
}

func newMailClientV2(f *Fs) (*mailclient2, error) {
	cli := &mailclient2{
		conn:       nil,
		f:          f,
		namespaces: []*namespace{},
	}
	// try and login
	err := cli.login()
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (m *mailclient2) getMailboxNamespace(mailbox string) *namespace {
	// find delimiter
	for _, ns := range m.namespaces {
		if ns.matchesMailbox(mailbox) {
			return ns
		}
	}
	return nil
}

func (m *mailclient2) getDirNamespace(dir string) *namespace {
	// find delimiter
	for _, ns := range m.namespaces {
		if ns.matchesDir(dir) {
			return ns
		}
	}
	return nil
}

func (m *mailclient2) login() error {
	var err error
	// dialer
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	// create address
	portStr := strconv.Itoa(m.f.port)
	address := net.JoinHostPort(m.f.host, portStr)
	// set options
	options := &imapclient.Options{
		TLSConfig: &tls.Config{
			ServerName:         m.f.host,
			InsecureSkipVerify: m.f.skipVerify,
		},
	}
	// create client
	if m.f.security == securityStartTLS {
		conn, err := dialer.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("failed to dial IMAP server: %w", err)
		}
		m.conn, err = imapclient.NewStartTLS(conn, options)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("failed to create StartTLS client: %w", err)
		}
	} else if m.f.security == securityTLS {
		options.TLSConfig.NextProtos = []string{"imap"}
		//
		conn, err := tls.DialWithDialer(dialer, "tcp", address, options.TLSConfig)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("failed to dial IMAP server: %w", err)
		}
		m.conn = imapclient.New(conn, options)
	} else if m.f.security == securityNone {
		m.conn, err = imapclient.DialInsecure(address, options)
		if err != nil {
			return fmt.Errorf("failed to dial IMAP server: %w", err)
		}
	}
	// connected ok, now login
	err = m.conn.Login(m.f.user, m.f.pass).Wait()
	if err != nil {
		defer m.Logout()
		return fmt.Errorf("failed to login: %w", err)
	}
	// get namespace data
	namespaces, err := m.conn.Namespace().Wait()
	if err != nil {
		defer m.Logout()
		return err
	}
	// get namespaces
	for _, curr := range namespaces.Personal {
		dir := strings.ReplaceAll(curr.Prefix, string(curr.Delim), "/")
		m.namespaces = append(m.namespaces, &namespace{nsType: "P", dir: dir, mailbox: curr.Prefix, delimiter: string(curr.Delim)})
	}
	for _, curr := range namespaces.Other {
		dir := strings.ReplaceAll(curr.Prefix, string(curr.Delim), "/")
		m.namespaces = append(m.namespaces, &namespace{nsType: "O", dir: dir, mailbox: curr.Prefix, delimiter: string(curr.Delim)})
	}
	for _, curr := range namespaces.Shared {
		dir := strings.ReplaceAll(curr.Prefix, string(curr.Delim), "/")
		m.namespaces = append(m.namespaces, &namespace{nsType: "S", dir: dir, mailbox: curr.Prefix, delimiter: string(curr.Delim)})
	}
	//
	return nil
}

func (m *mailclient2) Logout() {
	if m.conn != nil {
		_ = m.conn.Logout().Wait()
		_ = m.conn.Close()
	}
	//
	m.conn = nil
}

func (m *mailclient2) ListMailboxes(dir string) ([]string, error) {
	if m.conn == nil {
		return nil, fmt.Errorf("failed to list mailboxes : not connected")
	}
	// get mailboxes
	mailboxes, err := m.conn.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to get mailboxes: %w", err)
	}
	// save mailboxes
	list := []string{}
	for _, mbox := range mailboxes {
		ns := m.getMailboxNamespace(mbox.Mailbox)
		if ns != nil {
			list = append(list, ns.mailboxToDir(mbox.Mailbox))
		} else {
			fs.Debugf(nil, "failed to find namespace for mailbox %s", mbox.Mailbox)
		}
	}
	return getMatches(dir, list), nil
}

func (m *mailclient2) HasMailbox(dir string) bool {
	if m.conn == nil {
		return false
	}
	options := imap.StatusOptions{
		NumMessages: true,
	}
	// find namespace for dir
	ns := m.getDirNamespace(dir)
	if ns == nil {
		return false
	}
	//
	_, err := m.conn.Status(ns.dirToMailbox(dir), &options).Wait()
	return err == nil
}

func (m *mailclient2) RenameMailbox(from string, to string) error {
	if m.conn == nil {
		return fmt.Errorf("failed to rename mailbox %s: not connected", from)
	}
	// find namespace for source dir
	ns1 := m.getDirNamespace(from)
	if ns1 == nil {
		return fmt.Errorf("failed to get namespace for source %s", from)
	}
	// find namespace for destination dir
	ns2 := m.getDirNamespace(to)
	if ns2 == nil {
		return fmt.Errorf("failed to get namespace for destination %s", from)
	}
	options := imap.RenameOptions{}
	fs.Debugf(nil, "Rename mailbox %s to %s", from, to)
	err := m.conn.Rename(ns1.dirToMailbox(from), ns2.dirToMailbox(to), &options).Wait()
	if err != nil {
		return fmt.Errorf("failed to rename mailbox %s: %w", from, err)
	}
	return nil
}

func (m *mailclient2) CreateMailbox(name string) error {
	if m.conn == nil {
		return fmt.Errorf("failed to create mailbox %s: not connected", name)
	}
	// find namespace for dir
	ns := m.getDirNamespace(name)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", name)
	}
	//
	fs.Debugf(nil, "Create mailbox %s", name)
	err := m.conn.Create(ns.dirToMailbox(name), nil).Wait()
	if err != nil {
		return fmt.Errorf("failed to create mailbox %s: %w", name, err)
	}
	return nil
}

func (m *mailclient2) DeleteMailbox(name string) error {
	if m.conn == nil {
		return fmt.Errorf("failed to delete mailbox %s: not connected", name)
	} else if name == "" {
		return fmt.Errorf("cant remove root")
	}
	// find namespace for dir
	ns := m.getDirNamespace(name)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", name)
	}
	// select mailbox, readonly
	selectedMbox, err := m.conn.Select(ns.dirToMailbox(name), &imap.SelectOptions{ReadOnly: false}).Wait()
	if err != nil {
		return fs.ErrorDirNotFound
	}
	if selectedMbox.NumMessages != 0 {
		return fmt.Errorf("mailbox not empty, has %d messages", selectedMbox.NumMessages)
	}
	//
	fs.Debugf(nil, "Delete mailbox %s", name)
	err = m.conn.Delete(ns.dirToMailbox(name)).Wait()
	if err != nil {
		return fmt.Errorf("failed to delete mailbox %s: %w", name, err)
	}
	return nil
}

func (m *mailclient2) ExpungeMailbox(name string) error {
	if m.conn == nil {
		return fmt.Errorf("failed to expunge mailbox %s: not connected", name)
	}
	// find namespace for dir
	ns := m.getDirNamespace(name)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", name)
	}
	// select mailbox, writable
	_, err := m.conn.Select(ns.dirToMailbox(name), &imap.SelectOptions{ReadOnly: false}).Wait()
	if err != nil {
		return fs.ErrorDirNotFound
	}
	// expunge
	fs.Debugf(nil, "Expunge mailbox: %s", name)
	_, err = m.conn.Expunge().Collect()
	if err != nil {
		return fmt.Errorf("failed to expunge mailbox %s: %w", name, err)
	}
	//
	return nil
}

func (m *mailclient2) Save(mailbox string, date time.Time, size int64, reader io.Reader, flags []string) (err error) {
	if m.conn == nil {
		return fmt.Errorf("failed to save message to mailbox %s: not connected", mailbox)
	}
	// find namespace for dir
	ns := m.getDirNamespace(mailbox)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", mailbox)
	}
	// convert flags
	flagList := []imap.Flag{}
	for _, curr := range flags {
		flagList = append(flagList, imap.Flag(curr))
	}
	//
	opts := &imap.AppendOptions{
		Flags: flagList,
		Time:  date,
	}
	appendCommand := m.conn.Append(ns.dirToMailbox(mailbox), size, opts)
	// do copy
	_, copyErr := io.Copy(appendCommand, reader)
	closeErr := appendCommand.Close()
	_, waitErr := appendCommand.Wait()
	// something failed, either copt,close,or wait
	if copyErr != nil || closeErr != nil || waitErr != nil {
		return fmt.Errorf("failed to save message to mailbox %s: %w", mailbox, errors.Join(copyErr, closeErr, waitErr))
	}
	return nil
}

func (m *mailclient2) SetFlags(mailbox string, ids []uint32, flags ...string) error {
	if m.conn == nil {
		return fmt.Errorf("failed to sert message flags: not connected")
	}
	// find namespace for dir
	ns := m.getDirNamespace(mailbox)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", mailbox)
	}
	// select mailbox, writable
	_, err := m.conn.Select(ns.dirToMailbox(mailbox), &imap.SelectOptions{ReadOnly: false}).Wait()
	if err != nil {
		return fs.ErrorDirNotFound
	}
	// convert flags
	flagList := []imap.Flag{}
	for _, curr := range flags {
		flagList = append(flagList, imap.Flag(curr))
	}
	//
	storeFlags := imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Flags:  flagList,
		Silent: true,
	}
	fs.Debugf(nil, "Set flags for messages with ID [%s]: %s", strings.Fields(strings.Trim(fmt.Sprint(ids), "[]")), strings.Fields(strings.Trim(fmt.Sprint(flags), "[]")))
	err = m.conn.Store(imap.SeqSetNum(ids...), &storeFlags, nil).Close()
	if err != nil {
		return fmt.Errorf("failed to set flags: %w", err)
	}
	return nil
}

func (m *mailclient2) Search(mailbox string, since, before time.Time, larger, smaller uint32) (seqNums []uint32, err error) {
	if m.conn == nil {
		return nil, fmt.Errorf("failed to search messages : not connected")
	}
	// find namespace for dir
	ns := m.getDirNamespace(mailbox)
	if ns == nil {
		return nil, fmt.Errorf("failed to get namespace for %s", mailbox)
	}
	// select mailbox, readonly
	selectedMbox, err := m.conn.Select(ns.dirToMailbox(mailbox), &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}
	if selectedMbox.NumMessages == 0 {
		return []uint32{}, nil
	}
	//
	// init criteria for search
	criteria := &imap.SearchCriteria{}
	// include messages within date range
	criteria.Since = since
	criteria.Before = before
	// include messages with size between size-50 and size+50
	criteria.Larger = int64(larger)
	criteria.Smaller = int64(smaller)
	//
	data, err := m.conn.Search(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to search messages : %w", err)
	}
	return data.AllSeqNums(), err
}

func (m *mailclient2) forEachInternal(seqset imap.NumSet, action func(uint32, time.Time, uint32, []string, io.Reader)) error {
	var currFlags []string
	var currTime time.Time
	var currSize uint32
	var currReader io.ReadCloser
	// leave if no action
	if action == nil {
		return nil
	}
	// set fetch options
	fetchOptions := &imap.FetchOptions{
		Flags:        true,
		InternalDate: true,
		RFC822Size:   true,
		BodySection: []*imap.FetchItemBodySection{{
			Peek: true,
		}},
	}
	// fetch messages
	fetchCmd := m.conn.Fetch(seqset, fetchOptions)
	for {
		index := 0
		// get next message
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}
		// process message items
		currFlags = []string{}
		currTime = time.Now()
		currSize = 0
		currReader = nil
		for {
			item := msg.Next()
			if item == nil {
				break
			}
			//
			switch item := item.(type) {
			case imapclient.FetchItemDataFlags:
				for _, curr := range item.Flags {
					currFlags = append(currFlags, string(curr))
				}
			case imapclient.FetchItemDataInternalDate:
				currTime = item.Time
			case imapclient.FetchItemDataRFC822Size:
				currSize = uint32(item.Size)
			case imapclient.FetchItemDataBodySection:
				buf, currErr := io.ReadAll(item.Literal)
				if currErr == nil {
					currReader = io.NopCloser(bytes.NewReader(buf))
				}
			}
			index += 1
		}
		// call action
		if currReader != nil {
			action(msg.SeqNum, currTime, currSize, currFlags, currReader)
		} else {
			fs.Debugf(nil, "failed to process message [%d]: reader is null", msg.SeqNum)
		}
	}
	//
	return fetchCmd.Close()
}

func (m *mailclient2) ForEach(mailbox string, action func(uint32, time.Time, uint32, []string, io.Reader)) error {
	if m.conn == nil {
		return fmt.Errorf("failed to fetch messages : not connected")
	}
	// find namespace for dir
	ns := m.getDirNamespace(mailbox)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", mailbox)
	}
	// select mailbox, readonly
	selectedMbox, err := m.conn.Select(ns.dirToMailbox(mailbox), &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return fs.ErrorDirNotFound
	} else if selectedMbox.NumMessages == 0 {
		return nil
	}
	// set messages to fetch
	seqset := imap.SeqSetNum()
	seqset.AddRange(1, selectedMbox.NumMessages)
	// do fetch
	return m.forEachInternal(seqset, action)
}

func (m *mailclient2) ForEachID(mailbox string, ids []uint32, action func(uint32, time.Time, uint32, []string, io.Reader)) error {
	if m.conn == nil {
		return fmt.Errorf("failed to fetch messages : not connected")
	}
	// find namespace for dir
	ns := m.getDirNamespace(mailbox)
	if ns == nil {
		return fmt.Errorf("failed to get namespace for %s", mailbox)
	}
	// select mailbox, readonly
	selectedMbox, err := m.conn.Select(ns.dirToMailbox(mailbox), &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return fs.ErrorDirNotFound
	}
	if selectedMbox.NumMessages == 0 {
		return nil
	}
	// set messages to fetch
	seqset := imap.SeqSetNum(ids...)
	// do fetch
	return m.forEachInternal(seqset, action)
}
