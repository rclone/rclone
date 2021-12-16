package neofs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/cli/flags"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	resolver "github.com/nspcc-dev/neofs-sdk-go/ns"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
	"github.com/rclone/rclone/fs"
)

// EndpointInfo stores information about endpoint.
type EndpointInfo struct {
	Address  string
	Priority int
	Weight   float64
}

func parseEndpoints(endpointParam string) ([]EndpointInfo, error) {
	var err error
	expectedLength := -1 // to make sure all endpoints have the same format

	endpoints := strings.Split(strings.TrimSpace(endpointParam), " ")
	res := make([]EndpointInfo, 0, len(endpoints))
	seen := make(map[string]struct{}, len(endpoints))

	for _, endpoint := range endpoints {
		endpointInfoSplit := strings.Split(endpoint, ",")
		address := endpointInfoSplit[0]

		if len(address) == 0 {
			continue
		}
		if _, ok := seen[address]; ok {
			return nil, fmt.Errorf("endpoint '%s' is already defined", address)
		}
		seen[address] = struct{}{}

		endpointInfo := EndpointInfo{
			Address:  address,
			Priority: 1,
			Weight:   1,
		}

		if expectedLength == -1 {
			expectedLength = len(endpointInfoSplit)
		}

		if len(endpointInfoSplit) != expectedLength {
			return nil, fmt.Errorf("all endpoints must have the same format: '%s'", endpointParam)
		}

		switch len(endpointInfoSplit) {
		case 1:
		case 2:
			endpointInfo.Priority, err = parsePriority(endpointInfoSplit[1])
			if err != nil {
				return nil, fmt.Errorf("invalid endpoint '%s': %w", endpoint, err)
			}
		case 3:
			endpointInfo.Priority, err = parsePriority(endpointInfoSplit[1])
			if err != nil {
				return nil, fmt.Errorf("invalid endpoint '%s': %w", endpoint, err)
			}

			endpointInfo.Weight, err = parseWeight(endpointInfoSplit[2])
			if err != nil {
				return nil, fmt.Errorf("invalid endpoint '%s': %w", endpoint, err)
			}
		default:
			return nil, fmt.Errorf("invalid endpoint format '%s'", endpoint)
		}

		res = append(res, endpointInfo)
	}

	return res, nil
}

func parsePriority(priorityStr string) (int, error) {
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		return 0, fmt.Errorf("invalid priority '%s': %w", priorityStr, err)
	}
	if priority <= 0 {
		return 0, fmt.Errorf("priority must be positive '%s'", priorityStr)
	}

	return priority, nil
}

func parseWeight(weightStr string) (float64, error) {
	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid weight '%s': %w", weightStr, err)
	}
	if weight <= 0 {
		return 0, fmt.Errorf("weight must be positive '%s'", weightStr)
	}

	return weight, nil
}

func createPool(ctx context.Context, key *keys.PrivateKey, cfg *Options) (*pool.Pool, error) {
	var prm pool.InitParameters
	prm.SetKey(&key.PrivateKey)
	prm.SetNodeDialTimeout(time.Duration(cfg.NeofsDialTimeout))
	prm.SetHealthcheckTimeout(time.Duration(cfg.NeofsHealthcheckTimeout))
	prm.SetClientRebalanceInterval(time.Duration(cfg.NeofsRebalanceInterval))
	prm.SetSessionExpirationDuration(cfg.NeofsSessionExpirationDuration)

	nodes, err := getNodePoolParams(cfg.NeofsEndpoint)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		prm.AddNode(node)
	}

	p, err := pool.NewPool(prm)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err = p.Dial(ctx); err != nil {
		return nil, fmt.Errorf("dial pool: %w", err)
	}

	return p, nil
}

func getNodePoolParams(endpointParam string) ([]pool.NodeParam, error) {
	endpointInfos, err := parseEndpoints(endpointParam)
	if err != nil {
		return nil, fmt.Errorf("parse endpoints params: %w", err)
	}

	res := make([]pool.NodeParam, len(endpointInfos))
	for i, info := range endpointInfos {
		res[i] = pool.NewNodeParam(info.Priority, info.Address, info.Weight)
	}

	return res, nil
}

func createNNSResolver(cfg *Options) (*resolver.NNS, error) {
	if cfg.RPCEndpoint == "" {
		return nil, nil
	}

	var nns resolver.NNS
	if err := nns.Dial(cfg.RPCEndpoint); err != nil {
		return nil, fmt.Errorf("dial NNS resolver: %w", err)
	}

	return &nns, nil
}

func getAccount(cfg *Options) (*wallet.Account, error) {
	w, err := wallet.NewWalletFromFile(cfg.Wallet)
	if err != nil {
		return nil, err
	}

	addr := w.GetChangeAddress()
	if cfg.Address != "" {
		addr, err = flags.ParseAddress(cfg.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid address")
		}
	}
	acc := w.GetAccount(addr)
	err = acc.Decrypt(cfg.Password, w.Scrypt)
	if err != nil {
		return nil, err
	}

	return acc, nil
}

func newAddress(cnrID cid.ID, objID oid.ID) oid.Address {
	var addr oid.Address
	addr.SetContainer(cnrID)
	addr.SetObject(objID)
	return addr
}

func formObject(own *user.ID, cnrID cid.ID, header map[string]string) *object.Object {
	attributes := make([]object.Attribute, 0, len(header))

	for key, val := range header {
		attr := object.NewAttribute()
		attr.SetKey(key)
		attr.SetValue(val)
		attributes = append(attributes, *attr)
	}

	obj := object.New()
	obj.SetOwnerID(own)
	obj.SetContainerID(cnrID)
	obj.SetAttributes(attributes...)

	return obj
}

func newDir(cnrID cid.ID, cnr container.Container) *fs.Dir {
	remote := cnrID.EncodeToString()
	timestamp := container.CreatedAt(cnr)

	if domain := container.ReadDomain(cnr); domain.Name() != "" {
		remote = domain.Name()
	}

	dir := fs.NewDir(remote, timestamp)
	dir.SetID(cnrID.String())

	return dir
}
