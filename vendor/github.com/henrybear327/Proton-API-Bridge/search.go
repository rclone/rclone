package proton_api_bridge

import (
	"context"

	"github.com/henrybear327/go-proton-api"
)

/*
The filename is unique in a given folder, since it's checked (by using hash) on the server
*/

// if the target isn't found, nil will be returned for both return values
func (protonDrive *ProtonDrive) SearchByNameInActiveFolderByID(ctx context.Context,
	folderLinkID string,
	targetName string,
	searchForFile, searchForFolder bool,
	targetState proton.LinkState) (*proton.Link, error) {
	folderLink, err := protonDrive.getLink(ctx, folderLinkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.SearchByNameInActiveFolder(ctx, folderLink, targetName, searchForFile, searchForFolder, targetState)
}

func (protonDrive *ProtonDrive) SearchByNameInActiveFolder(
	ctx context.Context,
	folderLink *proton.Link,
	targetName string,
	searchForFile, searchForFolder bool,
	targetState proton.LinkState) (*proton.Link, error) {
	if !searchForFile && !searchForFolder {
		// nothing to search
		return nil, nil
	}

	// we search all folders and files within this designated folder only
	if folderLink.Type != proton.LinkTypeFolder {
		return nil, ErrLinkTypeMustToBeFolderType
	}

	if folderLink.State != proton.LinkStateActive {
		// we only search in the active folders
		return nil, nil
	}

	// get target name Hash
	parentNodeKR, err := protonDrive.getLinkKRByID(ctx, folderLink.ParentLinkID)
	if err != nil {
		return nil, err
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{folderLink.SignatureEmail})
	if err != nil {
		return nil, err
	}
	folderLinkKR, err := folderLink.GetKeyRing(parentNodeKR, signatureVerificationKR)
	if err != nil {
		return nil, err
	}

	signatureVerificationKR, err = protonDrive.getSignatureVerificationKeyring([]string{folderLink.SignatureEmail}, folderLinkKR)
	if err != nil {
		return nil, err
	}
	folderHashKey, err := folderLink.GetHashKey(folderLinkKR, signatureVerificationKR)
	if err != nil {
		return nil, err
	}

	targetNameHash, err := proton.GetNameHash(targetName, folderHashKey)
	if err != nil {
		return nil, err
	}

	// use available hash to check if it exists
	// more efficient than linear scan to just do existence check
	// used in rclone when Put(), it will try to see if the object exists or not
	res, err := protonDrive.c.CheckAvailableHashes(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, proton.CheckAvailableHashesReq{
		Hashes: []string{targetNameHash},
	})
	if err != nil {
		return nil, err
	}

	if len(res.AvailableHashes) == 1 {
		// name isn't taken == name doesn't exist
		return nil, nil
	}

	childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, true)
	if err != nil {
		return nil, err
	}
	for _, childLink := range childrenLinks {
		if childLink.State != targetState {
			continue
		}

		if searchForFile && childLink.Type == proton.LinkTypeFile && childLink.Hash == targetNameHash {
			return &childLink, nil
		} else if searchForFolder && childLink.Type == proton.LinkTypeFolder && childLink.Hash == targetNameHash {
			return &childLink, nil
		}
	}

	return nil, nil
}
