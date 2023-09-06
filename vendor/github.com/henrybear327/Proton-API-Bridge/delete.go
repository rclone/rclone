package proton_api_bridge

import (
	"context"

	"github.com/henrybear327/go-proton-api"
)

func (protonDrive *ProtonDrive) moveToTrash(ctx context.Context, parentLinkID string, linkIDs ...string) error {
	err := protonDrive.c.TrashChildren(ctx, protonDrive.MainShare.ShareID, parentLinkID, linkIDs...)
	if err != nil {
		return err
	}

	for _, link := range linkIDs {
		protonDrive.removeLinkIDFromCache(link, true)
	}

	return nil
}

func (protonDrive *ProtonDrive) MoveFileToTrashByID(ctx context.Context, linkID string) error {
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(linkID, false)

	fileLink, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return err
	}
	if fileLink.Type != proton.LinkTypeFile {
		return ErrLinkTypeMustToBeFileType
	}

	return protonDrive.moveToTrash(ctx, fileLink.ParentLinkID, linkID)
}

func (protonDrive *ProtonDrive) MoveFolderToTrashByID(ctx context.Context, linkID string, onlyOnEmpty bool) error {
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(linkID, false)

	folderLink, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return err
	}
	if folderLink.Type != proton.LinkTypeFolder {
		return ErrLinkTypeMustToBeFolderType
	}

	childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, linkID /* false: list only active ones */, false)
	if err != nil {
		return err
	}

	if onlyOnEmpty {
		if len(childrenLinks) > 0 {
			return ErrFolderIsNotEmpty
		}
	}

	return protonDrive.moveToTrash(ctx, folderLink.ParentLinkID, linkID)
}

// WARNING!!!!
// Everything in the root folder will be moved to trash
// Most likely only used for debugging when the key is messed up
func (protonDrive *ProtonDrive) EmptyRootFolder(ctx context.Context) error {
	protonDrive.ClearCache()

	links, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, protonDrive.MainShare.LinkID, true)
	if err != nil {
		return err
	}

	{
		linkIDs := make([]string, 0)
		for i := range links {
			if links[i].State == proton.LinkStateActive /* use TrashChildren */ {
				linkIDs = append(linkIDs, links[i].LinkID)
			}
		}

		err := protonDrive.c.TrashChildren(ctx, protonDrive.MainShare.ShareID, protonDrive.MainShare.LinkID, linkIDs...)
		if err != nil {
			return err
		}
	}

	{
		linkIDs := make([]string, 0)
		for i := range links {
			if links[i].State != proton.LinkStateActive {
				linkIDs = append(linkIDs, links[i].LinkID)
			}
		}

		err := protonDrive.c.DeleteChildren(ctx, protonDrive.MainShare.ShareID, protonDrive.MainShare.LinkID, linkIDs...)
		if err != nil {
			return err
		}
	}

	return nil
}

// Empty the trash
func (protonDrive *ProtonDrive) EmptyTrash(ctx context.Context) error {
	protonDrive.ClearCache()

	err := protonDrive.c.EmptyTrash(ctx, protonDrive.MainShare.ShareID)
	if err != nil {
		return err
	}

	return nil
}
