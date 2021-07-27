package drive

import (
	"bytes"
	"context"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/valyala/fastjson"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"gopkg.in/dc0d/tinykv.v4"

	"github.com/rclone/rclone/fs"
)

const (
	v3DriveListPath = "/drive/v3/files"
)

var errNoClients = errors.New("all clients are disabled")

func newRotateRoundTripper(ctx context.Context, opt *Options, serviceAccountCredentials []string) (*rotateRoundTripper, error) {
	var serviceAccounts = make([]*serviceAccount, len(serviceAccountCredentials))

	for i, serviceAccountCredential := range serviceAccountCredentials {
		sa, err := newServiceAccount(ctx, opt, serviceAccountCredential)

		if err != nil {
			return nil, err
		}

		serviceAccounts[i] = sa
	}

	pl := &projectList{}
	projectMap := make(map[string]*project)

	for _, sa := range serviceAccounts {
		p := projectMap[sa.credentials.ProjectID]
		if p == nil {
			p = &project{}
			projectMap[sa.credentials.ProjectID] = p
		}

		p.serviceAccounts = append(p.serviceAccounts, sa)
		p.activeAccounts = append(p.activeAccounts, 1)
		p.rateLimitErrorCount = append(p.rateLimitErrorCount, 0)
	}

	for _, p := range projectMap {
		pl.list = append(pl.list, p)
	}

	return &rotateRoundTripper{
		accountDisableTime:            opt.AccountDisableTime,
		projectList:                   pl,
		useMultipleAccountsForListing: opt.MultiAccountListing,
		kv:                            tinykv.New(2 * time.Minute),
	}, nil
}

type rotateRoundTripper struct {
	accountDisableTime            fs.Duration
	projectList                   *projectList
	kv                            tinykv.KV
	useMultipleAccountsForListing bool
}

type projectList struct {
	mtx    sync.Mutex
	lastID int
	list   []*project
}

type project struct {
	serviceAccounts     []*serviceAccount
	activeAccounts      []int32
	rateLimitErrorCount []int32
}

func (p *projectList) getProjectForServiceAccount(serviceAccount *serviceAccount) *project {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	for _, project := range p.list {
		if project.serviceAccounts[0].credentials.ProjectID == serviceAccount.credentials.ProjectID {
			return project
		}
	}

	return nil
}

func (p *projectList) getFirstServiceAccount() *serviceAccount {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	return p.list[0].serviceAccounts[0]
}

func (p *projectList) getServiceAccount() (*project, *serviceAccount, int) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	p.lastID++
	if p.lastID > len(p.list)-1 {
		p.lastID = 0
	}
	account, index := p.list[p.lastID].GetServiceAccount()

	return p.list[p.lastID], account, index
}

func (p *project) GetServiceAccount() (*serviceAccount, int) {
	var randomTest int
	if len(p.serviceAccounts) > 1 {
		randomTest = rand.Intn(len(p.serviceAccounts) - 1)
	} else {
		fs.Errorf(nil, "Warning! Project %v has only one ServiceAccount", p.serviceAccounts[0].credentials.ProjectID)
	}

	if p.isAccountEnabled(randomTest) {
		return p.serviceAccounts[randomTest], randomTest
	}

	for index, serviceAccount := range p.serviceAccounts {
		if p.isAccountEnabled(index) {
			return serviceAccount, index
		}
	}

	return nil, 0
}

func (p *project) isAccountEnabled(i int) (enabled bool) {
	return atomic.LoadInt32(&p.activeAccounts[i]) == 1
}

func (p *project) enableAccount(i int) (alreadyEnabled bool) {
	alreadyEnabled = atomic.SwapInt32(&p.activeAccounts[i], 1) == 1
	if alreadyEnabled {
		atomic.StoreInt32(&p.rateLimitErrorCount[i], 0)
	}
	return
}

func (p *project) disableAccount(i int) (alreadyDisabled bool) {
	return atomic.SwapInt32(&p.activeAccounts[i], 0) == 0
}

func newServiceAccount(ctx context.Context, opt *Options, serviceAccountCredentials string) (*serviceAccount, error) {
	config, client, err := getServiceAccountClient(ctx, opt, []byte(serviceAccountCredentials), true)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create oauth client from service account")
	}

	creds, err := google.CredentialsFromJSON(context.TODO(), []byte(serviceAccountCredentials), config.Scopes...)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create credentials for service account")
	}

	return &serviceAccount{
		client:      client,
		credentials: creds,
		config:      config,
	}, nil
}

type serviceAccount struct {
	client      *http.Client
	config      *jwt.Config
	credentials *google.Credentials
}

// RoundTrip implements the http.RoundTripper interface and takes a random account for the request
func (r *rotateRoundTripper) RoundTrip(request *http.Request) (response *http.Response, err error) {
	if isListRequest(request) {
		if r.useMultipleAccountsForListing {
			response, err = r.doListRequest(request)
		} else {
			response, err = r.projectList.getFirstServiceAccount().client.Do(request)
		}
	} else {
		p, sa, index := r.projectList.getServiceAccount()

		if sa == nil {
			return nil, errNoClients
		}

		response, err = sa.client.Do(request)

		if isError(response) {
			r.onError(p, index)
		}
	}
	return response, err
}

func isError(response *http.Response) bool {
	switch {
	case response == nil:
		fallthrough
	case response.StatusCode == http.StatusBadRequest,
		response.StatusCode == http.StatusForbidden,
		response.StatusCode == http.StatusNotFound:
		return true
	}

	return false
}

func (r *rotateRoundTripper) onError(project *project, index int) {
	errorCount := atomic.AddInt32(&project.rateLimitErrorCount[index], 1)
	if errorCount > 3 {
		r.onFailure(project, index)
	}
}

func (r *rotateRoundTripper) onFailure(project *project, index int) {
	if !project.isAccountEnabled(index) {
		return
	}

	fs.Debugf(nil, "Got a lot of errors disabling serviceAccount %d", index)
	project.disableAccount(index)

	// Enable Client after 1h for recheck
	go func() {
		// TODO This leaks the round tripper for an hour.
		//  Use timeout and contexts.
		//  Cancel the timeout if the context is cancelled (aka roundtripper closed)
		<-time.After(time.Duration(r.accountDisableTime))
		fs.Debugf(nil, "Reenabling serviceAccount %d", index)
		project.enableAccount(index)
	}()
}

func isListRequest(r *http.Request) bool {
	return r.URL.Path == v3DriveListPath
}

func (r *rotateRoundTripper) doListRequest(request *http.Request) (response *http.Response, err error) {
	var sa *serviceAccount
	var requestPageToken string
	requestPageToken = request.URL.Query().Get("requestPageToken")

	if requestPageToken != "" {
		previousAccount, ok := r.kv.Get(requestPageToken)

		if ok {
			sa = previousAccount.(*serviceAccount)
		}
	}

	if sa == nil {
		_, sa, _ = r.projectList.getServiceAccount()
	}

	response, err = sa.client.Do(request)

	nextPageToken, err := r.getNextPageTokenFromResponse(response)
	if err != nil {
		return nil, err
	}

	if r.kv.Put(nextPageToken, sa) != nil {
		return nil, err
	}

	return response, err
}

func (r *rotateRoundTripper) getNextPageTokenFromResponse(res *http.Response) (nextPageToken string, err error) {
	listResponseData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.Body.Close() != nil {
		return "", err
	}

	var p fastjson.Parser
	value, err := p.ParseBytes(listResponseData)
	if err != nil {
		return "", err
	}

	if value.Exists("nextPageToken") {
		nextPageToken = string(value.GetStringBytes("nextPageToken"))
	}

	res.Body = ioutil.NopCloser(bytes.NewReader(listResponseData))

	return nextPageToken, nil
}
