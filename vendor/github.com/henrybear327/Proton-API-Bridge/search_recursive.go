package proton_api_bridge

import (
	"context"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

func (protonDrive *ProtonDrive) searchByNameRecursivelyFromRoot(ctx context.Context, targetName string, isFolder bool, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	var linkType proton.LinkType
	if isFolder {
		linkType = proton.LinkTypeFolder
	} else {
		linkType = proton.LinkTypeFile
	}
	return protonDrive.performSearchByNameRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, targetName, linkType, listAllActiveOrDraftFiles)
}

// func (protonDrive *ProtonDrive) searchByNameRecursivelyByID(ctx context.Context, folderLinkID string, targetName string, isFolder bool, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
// 	folderLink, err := protonDrive.getLink(ctx, folderLinkID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	var linkType proton.LinkType
// 	if isFolder {
// 		linkType = proton.LinkTypeFolder
// 	} else {
// 		linkType = proton.LinkTypeFile
// 	}

// 	if folderLink.Type != proton.LinkTypeFolder {
// 		return nil, ErrLinkTypeMustToBeFolderType
// 	}
// 	folderKeyRing, err := protonDrive.getLinkKRByID(ctx, folderLink.ParentLinkID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return protonDrive.performSearchByNameRecursively(ctx, folderKeyRing, folderLink, targetName, linkType, listAllActiveOrDraftFiles)
// }

func (protonDrive *ProtonDrive) SearchByNameRecursively(ctx context.Context, folderLink *proton.Link, targetName string, isFolder bool, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	var linkType proton.LinkType
	if isFolder {
		linkType = proton.LinkTypeFolder
	} else {
		linkType = proton.LinkTypeFile
	}

	if folderLink.Type != proton.LinkTypeFolder {
		return nil, ErrLinkTypeMustToBeFolderType
	}
	folderKeyRing, err := protonDrive.getLinkKRByID(ctx, folderLink.ParentLinkID)
	if err != nil {
		return nil, err
	}
	return protonDrive.performSearchByNameRecursively(ctx, folderKeyRing, folderLink, targetName, linkType, listAllActiveOrDraftFiles)
}

func (protonDrive *ProtonDrive) performSearchByNameRecursively(
	ctx context.Context,
	parentNodeKR *crypto.KeyRing,
	link *proton.Link,
	targetName string,
	linkType proton.LinkType,
	listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	if listAllActiveOrDraftFiles {
		if link.State != proton.LinkStateActive && link.State != proton.LinkStateDraft {
			return nil, nil
		}
	} else if link.State != proton.LinkStateActive {
		return nil, nil
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.NameSignatureEmail, link.SignatureEmail})
	if err != nil {
		return nil, err
	}
	name, err := link.GetName(parentNodeKR, signatureVerificationKR)
	if err != nil {
		return nil, err
	}

	if link.Type == linkType && name == targetName {
		return link, nil
	}

	if link.Type == proton.LinkTypeFolder {
		childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, link.LinkID, true)
		if err != nil {
			return nil, err
		}
		// log.Printf("childrenLinks len = %v, %#v", len(childrenLinks), childrenLinks)

		// get current node's keyring
		signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.SignatureEmail})
		if err != nil {
			return nil, err
		}
		linkKR, err := link.GetKeyRing(parentNodeKR, signatureVerificationKR)
		if err != nil {
			return nil, err
		}

		for _, childLink := range childrenLinks {
			ret, err := protonDrive.performSearchByNameRecursively(ctx, linkKR, &childLink, targetName, linkType, listAllActiveOrDraftFiles)
			if err != nil {
				return nil, err
			}

			if ret != nil {
				return ret, nil
			}
		}
	}

	return nil, nil
}
