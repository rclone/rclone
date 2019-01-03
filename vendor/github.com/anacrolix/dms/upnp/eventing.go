package upnp

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"time"
)

// TODO: Why use namespace prefixes in PropertySet et al? Because the spec
// uses them, and I believe the Golang standard library XML spec implementers
// incorrectly assume that you can get away with just xmlns="".

// propertyset is the root element sent in an event callback.
type PropertySet struct {
	XMLName    struct{} `xml:"e:propertyset"`
	Properties []Property
	// This should be set to `"urn:schemas-upnp-org:event-1-0"`.
	Space string `xml:"xmlns:e,attr"`
}

// propertys provide namespacing to the contained variables.
type Property struct {
	XMLName  struct{} `xml:"e:property"`
	Variable Variable
}

// Represents an evented state variable that has sendEvents="yes" in its
// service spec.
type Variable struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type subscriber struct {
	sid     string
	nextSeq uint32 // 0 for initial event, wraps from Uint32Max to 1.
	urls    []*url.URL
	expiry  time.Time
}

// Intended to eventually be an embeddable implementation for managing
// eventing for a service. Not complete.
type Eventing struct {
	subscribers map[string]*subscriber
}

func (me *Eventing) Subscribe(callback []*url.URL, timeoutSeconds int) (sid string, actualTimeout int, err error) {
	var uuid [16]byte
	io.ReadFull(rand.Reader, uuid[:])
	sid = FormatUUID(uuid[:])
	if _, ok := me.subscribers[sid]; ok {
		err = fmt.Errorf("already subscribed: %s", sid)
		return
	}
	ssr := &subscriber{
		sid:    sid,
		urls:   callback,
		expiry: time.Now().Add(time.Duration(timeoutSeconds) * time.Second),
	}
	if me.subscribers == nil {
		me.subscribers = make(map[string]*subscriber)
	}
	me.subscribers[sid] = ssr
	actualTimeout = int(ssr.expiry.Sub(time.Now()) / time.Second)
	return
}

func (me *Eventing) Unsubscribe(sid string) error {
	return nil
}

var callbackURLRegexp = regexp.MustCompile("<(.*?)>")

// Parse the CALLBACK HTTP header in an event subscription request. See UPnP
// Device Architecture 4.1.2.
func ParseCallbackURLs(callback string) (ret []*url.URL) {
	for _, match := range callbackURLRegexp.FindAllStringSubmatch(callback, -1) {
		_url, err := url.Parse(match[1])
		if err != nil {
			log.Printf("bad callback url: %q", match[1])
			continue
		}
		ret = append(ret, _url)
	}
	return
}
