// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient

import (
	"context"
	"io"
	"io/ioutil"
	"sort"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/encryption"
	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/ranger"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/uplink/private/eestream"
	"storj.io/uplink/private/piecestore"
)

var mon = monkit.Package()

// Client defines an interface for storing erasure coded data to piece store nodes.
type Client interface {
	Put(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error)
	PutSingleResult(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (results []*pb.SegmentPieceUploadResult, err error)
	Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (ranger.Ranger, error)
	WithForceErrorDetection(force bool) Client
	// PutPiece is not intended to be used by normal uplinks directly, but is exported to support storagenode graceful exit transfers.
	PutPiece(ctx, parent context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, data io.ReadCloser) (hash *pb.PieceHash, id *identity.PeerIdentity, err error)
}

type dialPiecestoreFunc func(context.Context, storj.NodeURL) (*piecestore.Client, error)

type ecClient struct {
	log                 *zap.Logger
	dialer              rpc.Dialer
	memoryLimit         int
	forceErrorDetection bool
}

// NewClient from the given identity and max buffer memory.
func NewClient(log *zap.Logger, dialer rpc.Dialer, memoryLimit int) Client {
	return &ecClient{
		log:         log,
		dialer:      dialer,
		memoryLimit: memoryLimit,
	}
}

func (ec *ecClient) WithForceErrorDetection(force bool) Client {
	ec.forceErrorDetection = force
	return ec
}

func (ec *ecClient) dialPiecestore(ctx context.Context, n storj.NodeURL) (*piecestore.Client, error) {
	logger := ec.log.Named(n.ID.String())
	return piecestore.DialNodeURL(ctx, ec.dialer, n, logger, piecestore.DefaultConfig)
}

func (ec *ecClient) Put(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)

	pieceCount := len(limits)
	if pieceCount != rs.TotalCount() {
		return nil, nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", pieceCount, rs.TotalCount())
	}

	nonNilLimits := nonNilCount(limits)
	if nonNilLimits <= rs.RepairThreshold() && nonNilLimits < rs.OptimalThreshold() {
		return nil, nil, Error.New("number of non-nil limits (%d) is less than or equal to the repair threshold (%d) of erasure scheme", nonNilLimits, rs.RepairThreshold())
	}

	if !unique(limits) {
		return nil, nil, Error.New("duplicated nodes are not allowed")
	}

	ec.log.Debug("Uploading to storage nodes",
		zap.Int("Erasure Share Size", rs.ErasureShareSize()),
		zap.Int("Stripe Size", rs.StripeSize()),
		zap.Int("Repair Threshold", rs.RepairThreshold()),
		zap.Int("Optimal Threshold", rs.OptimalThreshold()),
	)

	padded := encryption.PadReader(ioutil.NopCloser(data), rs.StripeSize())
	readers, err := eestream.EncodeReader(ctx, ec.log, padded, rs)
	if err != nil {
		return nil, nil, err
	}

	type info struct {
		i    int
		err  error
		hash *pb.PieceHash
	}
	infos := make(chan info, pieceCount)

	psCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, addressedLimit := range limits {
		go func(i int, addressedLimit *pb.AddressedOrderLimit) {
			hash, _, err := ec.PutPiece(psCtx, ctx, addressedLimit, privateKey, readers[i])
			infos <- info{i: i, err: err, hash: hash}
		}(i, addressedLimit)
	}

	successfulNodes = make([]*pb.Node, pieceCount)
	successfulHashes = make([]*pb.PieceHash, pieceCount)
	var successfulCount, failureCount, cancellationCount int32
	for range limits {
		info := <-infos

		if limits[info.i] == nil {
			continue
		}

		if info.err != nil {
			if !errs2.IsCanceled(info.err) {
				failureCount++
			} else {
				cancellationCount++
			}
			ec.log.Debug("Upload to storage node failed",
				zap.Stringer("Node ID", limits[info.i].GetLimit().StorageNodeId),
				zap.Error(info.err),
			)
			continue
		}

		successfulNodes[info.i] = &pb.Node{
			Id:      limits[info.i].GetLimit().StorageNodeId,
			Address: limits[info.i].GetStorageNodeAddress(),
		}
		successfulHashes[info.i] = info.hash

		successfulCount++
		if int(successfulCount) >= rs.OptimalThreshold() {
			ec.log.Debug("Success threshold reached. Cancelling remaining uploads.",
				zap.Int("Optimal Threshold", rs.OptimalThreshold()),
			)
			cancel()
		}
	}

	defer func() {
		select {
		case <-ctx.Done():
			err = Error.New("upload cancelled by user")
		default:
		}
	}()

	mon.IntVal("put_segment_pieces_total").Observe(int64(pieceCount))
	mon.IntVal("put_segment_pieces_optimal").Observe(int64(rs.OptimalThreshold()))
	mon.IntVal("put_segment_pieces_successful").Observe(int64(successfulCount))
	mon.IntVal("put_segment_pieces_failed").Observe(int64(failureCount))
	mon.IntVal("put_segment_pieces_canceled").Observe(int64(cancellationCount))

	if int(successfulCount) <= rs.RepairThreshold() && int(successfulCount) < rs.OptimalThreshold() {
		return nil, nil, Error.New("successful puts (%d) less than or equal to repair threshold (%d)", successfulCount, rs.RepairThreshold())
	}

	if int(successfulCount) < rs.OptimalThreshold() {
		return nil, nil, Error.New("successful puts (%d) less than success threshold (%d)", successfulCount, rs.OptimalThreshold())
	}

	return successfulNodes, successfulHashes, nil
}

func (ec *ecClient) PutSingleResult(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, rs eestream.RedundancyStrategy, data io.Reader, expiration time.Time) (results []*pb.SegmentPieceUploadResult, err error) {
	successfulNodes, successfulHashes, err := ec.Put(ctx, limits, privateKey, rs, data, expiration)
	if err != nil {
		return nil, err
	}

	uploadResults := make([]*pb.SegmentPieceUploadResult, 0, len(successfulNodes))
	for i := range successfulNodes {
		if successfulNodes[i] == nil {
			continue
		}

		uploadResults = append(uploadResults, &pb.SegmentPieceUploadResult{
			PieceNum: int32(i),
			NodeId:   successfulNodes[i].Id,
			Hash:     successfulHashes[i],
		})
	}

	if l := len(uploadResults); l < rs.OptimalThreshold() {
		return nil, Error.New("uploaded results (%d) are below the optimal threshold (%d)", l, rs.OptimalThreshold())
	}

	return uploadResults, nil
}

func (ec *ecClient) PutPiece(ctx, parent context.Context, limit *pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, data io.ReadCloser) (hash *pb.PieceHash, peerID *identity.PeerIdentity, err error) {
	nodeName := "nil"
	if limit != nil {
		nodeName = limit.GetLimit().StorageNodeId.String()[0:8]
	}
	defer mon.Task()(&ctx, "node: "+nodeName)(&err)
	defer func() { err = errs.Combine(err, data.Close()) }()

	if limit == nil {
		_, _ = io.Copy(ioutil.Discard, data)
		return nil, nil, nil
	}

	storageNodeID := limit.GetLimit().StorageNodeId
	pieceID := limit.GetLimit().PieceId
	ps, err := ec.dialPiecestore(ctx, storj.NodeURL{
		ID:      storageNodeID,
		Address: limit.GetStorageNodeAddress().Address,
	})
	if err != nil {
		ec.log.Debug("Failed dialing for putting piece to node",
			zap.Stringer("Piece ID", pieceID),
			zap.Stringer("Node ID", storageNodeID),
			zap.Error(err),
		)
		return nil, nil, err
	}
	defer func() { err = errs.Combine(err, ps.Close()) }()

	peerID, err = ps.GetPeerIdentity()
	if err != nil {
		ec.log.Debug("Failed getting peer identity from node connection",
			zap.Stringer("Node ID", storageNodeID),
			zap.Error(err),
		)
		return nil, nil, err
	}

	hash, err = ps.UploadReader(ctx, limit.GetLimit(), privateKey, data)
	if err != nil {
		if ctx.Err() == context.Canceled {
			// Canceled context means the piece upload was interrupted by user or due
			// to slow connection. No error logging for this case.
			if parent.Err() == context.Canceled {
				ec.log.Info("Upload to node canceled by user", zap.Stringer("Node ID", storageNodeID))
			} else {
				ec.log.Debug("Node cut from upload due to slow connection", zap.Stringer("Node ID", storageNodeID))
			}

			// make sure context.Canceled is the primary error in the error chain
			// for later errors.Is/errs2.IsCanceled checking
			err = errs.Combine(context.Canceled, err)

		} else {
			nodeAddress := ""
			if limit.GetStorageNodeAddress() != nil {
				nodeAddress = limit.GetStorageNodeAddress().GetAddress()
			}

			ec.log.Debug("Failed uploading piece to node",
				zap.Stringer("Piece ID", pieceID),
				zap.Stringer("Node ID", storageNodeID),
				zap.String("Node Address", nodeAddress),
				zap.Error(err),
			)
		}

		return nil, nil, err
	}

	return hash, peerID, nil
}

func (ec *ecClient) Get(ctx context.Context, limits []*pb.AddressedOrderLimit, privateKey storj.PiecePrivateKey, es eestream.ErasureScheme, size int64) (rr ranger.Ranger, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(limits) != es.TotalCount() {
		return nil, Error.New("size of limits slice (%d) does not match total count (%d) of erasure scheme", len(limits), es.TotalCount())
	}

	if nonNilCount(limits) < es.RequiredCount() {
		return nil, Error.New("number of non-nil limits (%d) is less than required count (%d) of erasure scheme", nonNilCount(limits), es.RequiredCount())
	}

	paddedSize := calcPadded(size, es.StripeSize())
	pieceSize := paddedSize / int64(es.RequiredCount())

	rrs := map[int]ranger.Ranger{}
	for i, addressedLimit := range limits {
		if addressedLimit == nil {
			continue
		}

		rrs[i] = &lazyPieceRanger{
			dialPiecestore: ec.dialPiecestore,
			limit:          addressedLimit,
			privateKey:     privateKey,
			size:           pieceSize,
		}
	}

	rr, err = eestream.Decode(ec.log, rrs, es, ec.memoryLimit, ec.forceErrorDetection)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	ranger, err := encryption.Unpad(rr, int(paddedSize-size))
	return ranger, Error.Wrap(err)
}

func unique(limits []*pb.AddressedOrderLimit) bool {
	if len(limits) < 2 {
		return true
	}
	ids := make(storj.NodeIDList, len(limits))
	for i, addressedLimit := range limits {
		if addressedLimit != nil {
			ids[i] = addressedLimit.GetLimit().StorageNodeId
		}
	}

	// sort the ids and check for identical neighbors
	sort.Sort(ids)
	// sort.Slice(ids, func(i, k int) bool { return ids[i].Less(ids[k]) })
	for i := 1; i < len(ids); i++ {
		if ids[i] != (storj.NodeID{}) && ids[i] == ids[i-1] {
			return false
		}
	}

	return true
}

func calcPadded(size int64, blockSize int) int64 {
	mod := size % int64(blockSize)
	if mod == 0 {
		return size
	}
	return size + int64(blockSize) - mod
}

type lazyPieceRanger struct {
	dialPiecestore dialPiecestoreFunc
	limit          *pb.AddressedOrderLimit
	privateKey     storj.PiecePrivateKey
	size           int64
}

// Size implements Ranger.Size.
func (lr *lazyPieceRanger) Size() int64 {
	return lr.size
}

// Range implements Ranger.Range to be lazily connected.
func (lr *lazyPieceRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)

	return &lazyPieceReader{
		ranger: lr,
		ctx:    ctx,
		offset: offset,
		length: length,
	}, nil
}

type lazyPieceReader struct {
	ranger *lazyPieceRanger
	ctx    context.Context
	offset int64
	length int64

	mu sync.Mutex

	isClosed bool
	piecestore.Downloader
	client *piecestore.Client
}

func (lr *lazyPieceReader) Read(data []byte) (_ int, err error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.isClosed {
		return 0, io.EOF
	}
	if lr.Downloader == nil {
		client, downloader, err := lr.ranger.dial(lr.ctx, lr.offset, lr.length)
		if err != nil {
			return 0, err
		}
		lr.Downloader = downloader
		lr.client = client
	}

	return lr.Downloader.Read(data)
}

func (lr *lazyPieceRanger) dial(ctx context.Context, offset, length int64) (_ *piecestore.Client, _ piecestore.Downloader, err error) {
	defer mon.Task()(&ctx)(&err)
	ps, err := lr.dialPiecestore(ctx, storj.NodeURL{
		ID:      lr.limit.GetLimit().StorageNodeId,
		Address: lr.limit.GetStorageNodeAddress().Address,
	})
	if err != nil {
		return nil, nil, err
	}

	download, err := ps.Download(ctx, lr.limit.GetLimit(), lr.privateKey, offset, length)
	if err != nil {
		return nil, nil, errs.Combine(err, ps.Close())
	}
	return ps, download, nil
}

func (lr *lazyPieceReader) Close() (err error) {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.isClosed {
		return nil
	}
	lr.isClosed = true

	if lr.Downloader != nil {
		err = errs.Combine(err, lr.Downloader.Close())
	}
	if lr.client != nil {
		err = errs.Combine(err, lr.client.Close())
	}
	return err
}

func nonNilCount(limits []*pb.AddressedOrderLimit) int {
	total := 0
	for _, limit := range limits {
		if limit != nil {
			total++
		}
	}
	return total
}
