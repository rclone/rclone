package proton_api_bridge

import (
	"context"
	"time"

	"github.com/henrybear327/go-proton-api"
	"github.com/relvacode/iso8601"
)

type FileSystemAttrs struct {
	ModificationTime time.Time
	Size             int64
	BlockSizes       []int64
	Digests          string // sha1 string
}

func (protonDrive *ProtonDrive) GetRevisions(ctx context.Context, link *proton.Link, revisionType proton.RevisionState) ([]*proton.RevisionMetadata, error) {
	revisions, err := protonDrive.c.ListRevisions(ctx, protonDrive.MainShare.ShareID, link.LinkID)
	if err != nil {
		return nil, err
	}

	ret := make([]*proton.RevisionMetadata, 0)
	// Revisions are only for files, they represent “versions” of files.
	// Each file can have 1 active/draft revision and n obsolete revisions.
	for i := range revisions {
		if revisions[i].State == revisionType {
			ret = append(ret, &revisions[i])
		}
	}

	return ret, nil
}

func (protonDrive *ProtonDrive) GetActiveRevisionAttrsByID(ctx context.Context, linkID string) (*FileSystemAttrs, error) {
	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.GetActiveRevisionAttrs(ctx, link)
}

// Might return nil when xattr is missing
func (protonDrive *ProtonDrive) GetActiveRevisionAttrs(ctx context.Context, link *proton.Link) (*FileSystemAttrs, error) {
	if link == nil {
		return nil, ErrLinkMustNotBeNil
	}

	revisionsMetadata, err := protonDrive.GetRevisions(ctx, link, proton.RevisionStateActive)
	if err != nil {
		return nil, err
	}

	if len(revisionsMetadata) != 1 {
		return nil, ErrCantFindActiveRevision
	}

	nodeKR, err := protonDrive.getLinkKR(ctx, link)
	if err != nil {
		return nil, err
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.FileProperties.ActiveRevision.SignatureEmail})
	if err != nil {
		return nil, err
	}
	revisionXAttrCommon, err := revisionsMetadata[0].GetDecXAttrString(signatureVerificationKR, nodeKR)
	if err != nil {
		return nil, err
	}

	if revisionXAttrCommon == nil {
		return nil, nil
	}

	modificationTime, err := iso8601.ParseString(revisionXAttrCommon.ModificationTime)
	if err != nil {
		return nil, err
	}

	var sha1Hash string
	if val, ok := revisionXAttrCommon.Digests["SHA1"]; ok {
		sha1Hash = val
	} else {
		sha1Hash = ""
	}

	return &FileSystemAttrs{
		ModificationTime: modificationTime,
		Size:             revisionXAttrCommon.Size,
		BlockSizes:       revisionXAttrCommon.BlockSizes,
		Digests:          sha1Hash,
	}, nil
}

func (protonDrive *ProtonDrive) GetActiveRevisionWithAttrs(ctx context.Context, link *proton.Link) (*proton.Revision, *FileSystemAttrs, error) {
	if link == nil {
		return nil, nil, ErrLinkMustNotBeNil
	}

	revisionsMetadata, err := protonDrive.GetRevisions(ctx, link, proton.RevisionStateActive)
	if err != nil {
		return nil, nil, err
	}

	if len(revisionsMetadata) != 1 {
		return nil, nil, ErrCantFindActiveRevision
	}

	revision, err := protonDrive.c.GetRevisionAllBlocks(ctx, protonDrive.MainShare.ShareID, link.LinkID, revisionsMetadata[0].ID)
	if err != nil {
		return nil, nil, err
	}

	nodeKR, err := protonDrive.getLinkKR(ctx, link)
	if err != nil {
		return nil, nil, err
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.FileProperties.ActiveRevision.SignatureEmail})
	if err != nil {
		return nil, nil, err
	}
	revisionXAttrCommon, err := revision.GetDecXAttrString(signatureVerificationKR, nodeKR)
	if err != nil {
		return nil, nil, err
	}

	modificationTime, err := iso8601.ParseString(revisionXAttrCommon.ModificationTime)
	if err != nil {
		return nil, nil, err
	}

	var sha1Hash string
	if val, ok := revisionXAttrCommon.Digests["SHA1"]; ok {
		sha1Hash = val
	} else {
		sha1Hash = ""
	}

	return &revision, &FileSystemAttrs{
		ModificationTime: modificationTime,
		Size:             revisionXAttrCommon.Size,
		BlockSizes:       revisionXAttrCommon.BlockSizes,
		Digests:          sha1Hash,
	}, nil
}
