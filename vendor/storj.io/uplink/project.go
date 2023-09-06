// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/leak"
	"storj.io/common/memory"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/uplink/private/ecclient"
	"storj.io/uplink/private/metaclient"
	"storj.io/uplink/private/storage/streams"
	"storj.io/uplink/private/testuplink"
	"storj.io/uplink/private/version"
)

// TODO we need find a way how to pass it from satellite to client.
const maxInlineSize = 4096 // 4KiB

// maxSegmentSize can be used to override max segment size with ldflags build parameter.
// Example: go build -ldflags "-X 'storj.io/uplink.maxSegmentSize=1MiB'" storj.io/storj/cmd/uplink.
var maxSegmentSize string

// Project provides access to managing buckets and objects.
type Project struct {
	config                        Config
	access                        *Access
	satelliteDialer               rpc.Dialer
	storagenodeDialer             rpc.Dialer
	ec                            ecclient.Client
	segmentSize                   int64
	encryptionParameters          storj.EncryptionParameters
	concurrentSegmentUploadConfig *testuplink.ConcurrentSegmentUploadsConfig

	tracker leak.Ref
}

// OpenProject opens a project with the specific access grant.
func OpenProject(ctx context.Context, access *Access) (*Project, error) {
	return (Config{}).OpenProject(ctx, access)
}

// OpenProject opens a project with the specific access grant.
func (config Config) OpenProject(ctx context.Context, access *Access) (project *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	if access == nil {
		return nil, packageError.New("access grant is nil")
	}

	switch {
	case config.DialTimeout < 0:
		config.DialTimeout = 0 // no timeout
	case config.DialTimeout == 0:
		config.DialTimeout = defaultDialTimeout
	}

	if err := config.validateUserAgent(ctx); err != nil {
		return nil, packageError.New("invalid user agent: %w", err)
	}

	config.UserAgent, err = version.AppendVersionToUserAgent(config.UserAgent)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	storagenodeDialer, err := config.getDialerForPool(ctx, config.pool)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	satelliteDialer, err := config.getDialerForPool(ctx, config.satellitePool)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	// TODO: This should come from the EncryptionAccess. For now it's hardcoded to twice the
	// stripe size of the default redundancy scheme on the satellite.
	encBlockSize := 29 * 256 * memory.B.Int32()

	encryptionParameters := storj.EncryptionParameters{
		// N.B.: This is the ciphersuite we use for encrypting content keys,
		// which should absolutely be encrypted, even if the access grant
		// says EncNull.
		CipherSuite: storj.EncAESGCM,
		BlockSize:   encBlockSize,
	}

	// TODO: All these should be controlled by the satellite and not configured by the uplink.
	// For now we need to have these hard coded values that match the satellite configuration
	// to be able to create the underlying stream store.
	var (
		segmentsSize = 64 * memory.MiB.Int64()
	)

	if maxSegmentSize != "" {
		segmentsSize, err = memory.ParseString(maxSegmentSize)
		if err != nil {
			return nil, packageError.Wrap(err)
		}
	} else {
		s, ok := testuplink.GetMaxSegmentSize(ctx)
		if ok {
			segmentsSize = s.Int64()
		}
	}

	ec := ecclient.New(storagenodeDialer, 0)

	tracker := leak.FromContext(ctx)
	if tracker == (leak.Ref{}) { // TODO: handle this check better
		tracker = leak.Root(1)
	}

	return &Project{
		config:                        config,
		access:                        access,
		satelliteDialer:               satelliteDialer,
		storagenodeDialer:             storagenodeDialer,
		ec:                            ec,
		segmentSize:                   segmentsSize,
		encryptionParameters:          encryptionParameters,
		concurrentSegmentUploadConfig: testuplink.GetConcurrentSegmentUploadsConfig(ctx),

		tracker: tracker,
	}, nil
}

// Close closes the project and all associated resources.
func (project *Project) Close() (err error) {
	// only close the connection pools if it's created through OpenProject / getDialer()
	if project.config.pool == nil {
		err = errs.Combine(err, project.storagenodeDialer.Pool.Close())

		if project.config.satellitePool == nil {
			// if config.satellitePool is nil, but config.pool is not, it might be a second Close, but it's safe.
			err = errs.Combine(err, project.satelliteDialer.Pool.Close())
		}
	}

	return packageError.Wrap(errs.Combine(err, project.tracker.Close()))
}

func (project *Project) getStreamsStore(ctx context.Context) (_ *streams.Store, err error) {
	defer mon.Task()(&ctx)(&err)

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return nil, packageError.Wrap(err)
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, metainfoClient.Close())
		}
	}()

	var longTailMargin int
	if project.concurrentSegmentUploadConfig != nil {
		longTailMargin = project.concurrentSegmentUploadConfig.LongTailMargin
	}

	streamStore, err := streams.NewStreamStore(
		metainfoClient,
		project.ec,
		project.segmentSize,
		project.access.encAccess.Store,
		project.encryptionParameters,
		maxInlineSize,
		longTailMargin)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	return streamStore, nil
}

func (project *Project) dialMetainfoDB(ctx context.Context) (_ *metaclient.DB, err error) {
	defer mon.Task()(&ctx)(&err)

	metainfoClient, err := project.dialMetainfoClient(ctx)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	return metaclient.New(metainfoClient, project.access.encAccess.Store), nil
}

func (project *Project) dialMetainfoClient(ctx context.Context) (_ *metaclient.Client, err error) {
	defer mon.Task()(&ctx)(&err)

	metainfoClient, err := metaclient.DialNodeURL(ctx,
		project.satelliteDialer,
		project.access.satelliteURL.String(),
		project.access.apiKey,
		project.config.UserAgent)
	if err != nil {
		return nil, packageError.Wrap(err)
	}

	return metainfoClient, nil
}

//nolint:deadcode
//lint:ignore U1000 its used in private/object package
func dialMetainfoDBWithProject(ctx context.Context, project *Project) (_ *metaclient.DB, err error) {
	defer mon.Task()(&ctx)(&err)

	return project.dialMetainfoDB(ctx)
}

//nolint:deadcode
//lint:ignore U1000 its used in private/object package
func getStreamsStoreWithProject(ctx context.Context, project *Project) (_ *streams.Store, err error) {
	defer mon.Task()(&ctx)(&err)

	return project.getStreamsStore(ctx)
}
