/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

package dropbox

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"
)

type row struct {
	TID   string `json:"tid"`
	RowID string `json:"rowid"`
	Data  Fields `json:"data"`
}

type infoDict struct {
	Title string `json:"title"`
	MTime struct {
		Time DBTime `json:"T"`
	} `json:"mtime"`
}

type datastoreInfo struct {
	ID       string   `json:"dsid"`
	Handle   string   `json:"handle"`
	Revision int      `json:"rev"`
	Info     infoDict `json:"info"`
}

func (db *Dropbox) openOrCreateDatastore(dsID string) (int, string, bool, error) {
	var r struct {
		Revision int    `json:"rev"`
		Handle   string `json:"handle"`
		Created  bool   `json:"created"`
	}

	err := db.doRequest("POST", "datastores/get_or_create_datastore", &url.Values{"dsid": {dsID}}, &r)
	return r.Revision, r.Handle, r.Created, err
}

func (db *Dropbox) listDatastores() ([]DatastoreInfo, string, error) {
	var rv []DatastoreInfo

	var dl struct {
		Info  []datastoreInfo `json:"datastores"`
		Token string          `json:"token"`
	}

	if err := db.doRequest("GET", "datastores/list_datastores", nil, &dl); err != nil {
		return nil, "", err
	}
	rv = make([]DatastoreInfo, len(dl.Info))
	for i, di := range dl.Info {
		rv[i] = DatastoreInfo{
			ID:       di.ID,
			handle:   di.Handle,
			revision: di.Revision,
			title:    di.Info.Title,
			mtime:    time.Time(di.Info.MTime.Time),
		}
	}
	return rv, dl.Token, nil
}

func (db *Dropbox) deleteDatastore(handle string) (*string, error) {
	var r struct {
		NotFound string `json:"notfound"`
		OK       string `json:"ok"`
	}

	if err := db.doRequest("POST", "datastores/delete_datastore", &url.Values{"handle": {handle}}, &r); err != nil {
		return nil, err
	}
	if len(r.NotFound) != 0 {
		return nil, fmt.Errorf(r.NotFound)
	}
	return &r.OK, nil
}

func generateDatastoreID() (string, error) {
	var b []byte
	var blen int

	b = make([]byte, 1)
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		return "", err
	}
	blen = (int(b[0]) % maxGlobalIDLength) + 1
	b = make([]byte, blen)
	_, err = io.ReadFull(rand.Reader, b)
	if err != nil {
		return "", err
	}

	return encodeDBase64(b), nil
}

func (db *Dropbox) createDatastore(key string) (int, string, bool, error) {
	var r struct {
		Revision int    `json:"rev"`
		Handle   string `json:"handle"`
		Created  bool   `json:"created"`
		NotFound string `json:"notfound"`
	}
	var b64key string
	var err error

	if len(key) != 0 {
		b64key = encodeDBase64([]byte(key))
	} else {
		b64key, err = generateDatastoreID()
		if err != nil {
			return 0, "", false, err
		}
	}
	hash := sha256.New()
	hash.Write([]byte(b64key))
	rhash := hash.Sum(nil)
	dsID := "." + encodeDBase64(rhash[:])

	params := &url.Values{
		"key":  {b64key},
		"dsid": {dsID},
	}
	if err := db.doRequest("POST", "datastores/create_datastore", params, &r); err != nil {
		return 0, "", false, err
	}
	if len(r.NotFound) != 0 {
		return 0, "", false, fmt.Errorf("%s", r.NotFound)
	}
	return r.Revision, r.Handle, r.Created, nil
}

func (db *Dropbox) putDelta(handle string, rev int, changes listOfChanges) (int, error) {
	var r struct {
		Revision int    `json:"rev"`
		NotFound string `json:"notfound"`
		Conflict string `json:"conflict"`
		Error    string `json:"error"`
	}
	var js []byte
	var err error

	if len(changes) == 0 {
		return rev, nil
	}

	if js, err = json.Marshal(changes); err != nil {
		return 0, err
	}

	params := &url.Values{
		"handle":  {handle},
		"rev":     {strconv.FormatInt(int64(rev), 10)},
		"changes": {string(js)},
	}

	if err = db.doRequest("POST", "datastores/put_delta", params, &r); err != nil {
		return 0, err
	}
	if len(r.NotFound) != 0 {
		return 0, fmt.Errorf("%s", r.NotFound)
	}
	if len(r.Conflict) != 0 {
		return 0, fmt.Errorf("%s", r.Conflict)
	}
	if len(r.Error) != 0 {
		return 0, fmt.Errorf("%s", r.Error)
	}
	return r.Revision, nil
}

func (db *Dropbox) getDelta(handle string, rev int) ([]datastoreDelta, error) {
	var rv struct {
		Deltas   []datastoreDelta `json:"deltas"`
		NotFound string           `json:"notfound"`
	}
	err := db.doRequest("GET", "datastores/get_deltas",
		&url.Values{
			"handle": {handle},
			"rev":    {strconv.FormatInt(int64(rev), 10)},
		}, &rv)

	if len(rv.NotFound) != 0 {
		return nil, fmt.Errorf("%s", rv.NotFound)
	}
	return rv.Deltas, err
}

func (db *Dropbox) getSnapshot(handle string) ([]row, int, error) {
	var r struct {
		Rows     []row  `json:"rows"`
		Revision int    `json:"rev"`
		NotFound string `json:"notfound"`
	}

	if err := db.doRequest("GET", "datastores/get_snapshot",
		&url.Values{"handle": {handle}}, &r); err != nil {
		return nil, 0, err
	}
	if len(r.NotFound) != 0 {
		return nil, 0, fmt.Errorf("%s", r.NotFound)
	}
	return r.Rows, r.Revision, nil
}

func (db *Dropbox) await(cursors []*Datastore, token string) (string, []DatastoreInfo, map[string][]datastoreDelta, error) {
	var params *url.Values
	var dis []DatastoreInfo
	var dd map[string][]datastoreDelta

	type awaitResult struct {
		Deltas struct {
			Results map[string]struct {
				Deltas   []datastoreDelta `json:"deltas"`
				NotFound string           `json:"notfound"`
			} `json:"deltas"`
		} `json:"get_deltas"`
		Datastores struct {
			Info  []datastoreInfo `json:"datastores"`
			Token string          `json:"token"`
		} `json:"list_datastores"`
	}
	var r awaitResult
	if len(token) == 0 && len(cursors) == 0 {
		return "", nil, nil, fmt.Errorf("at least one parameter required")
	}
	params = &url.Values{}
	if len(token) != 0 {
		js, err := json.Marshal(map[string]string{"token": token})
		if err != nil {
			return "", nil, nil, err
		}
		params.Set("list_datastores", string(js))
	}
	if len(cursors) != 0 {
		m := make(map[string]int)
		for _, ds := range cursors {
			m[ds.info.handle] = ds.info.revision
		}
		js, err := json.Marshal(map[string]map[string]int{"cursors": m})
		if err != nil {
			return "", nil, nil, err
		}
		params.Set("get_deltas", string(js))
	}
	if err := db.doRequest("GET", "datastores/await", params, &r); err != nil {
		return "", nil, nil, err
	}
	if len(r.Deltas.Results) == 0 && len(r.Datastores.Info) == 0 {
		return token, nil, nil, fmt.Errorf("await timed out")
	}
	if len(r.Datastores.Token) != 0 {
		token = r.Datastores.Token
	}
	if len(r.Deltas.Results) != 0 {
		dd = make(map[string][]datastoreDelta)
		for k, v := range r.Deltas.Results {
			dd[k] = v.Deltas
		}
	}
	if len(r.Datastores.Info) != 0 {
		dis = make([]DatastoreInfo, len(r.Datastores.Info))
		for i, di := range r.Datastores.Info {
			dis[i] = DatastoreInfo{
				ID:       di.ID,
				handle:   di.Handle,
				revision: di.Revision,
				title:    di.Info.Title,
				mtime:    time.Time(di.Info.MTime.Time),
			}
		}
	}
	return token, dis, dd, nil
}
