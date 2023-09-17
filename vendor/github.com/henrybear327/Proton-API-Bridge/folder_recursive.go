package proton_api_bridge

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

func (protonDrive *ProtonDrive) listDirectoriesRecursively(
	ctx context.Context,
	parentNodeKR *crypto.KeyRing,
	link *proton.Link,
	download bool,
	maxDepth, curDepth /* 0-based */ int,
	excludeRoot bool,
	pathSoFar string,
	paths *[]string) error {
	/*
		Assumptions:
		- we only care about the active ones
	*/
	if link.State != proton.LinkStateActive {
		return nil
	}
	// log.Println("curDepth", curDepth, "pathSoFar", pathSoFar)

	var currentPath = ""

	if !(excludeRoot && curDepth == 0) {
		signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.NameSignatureEmail, link.SignatureEmail})
		if err != nil {
			return err
		}
		name, err := link.GetName(parentNodeKR, signatureVerificationKR)
		if err != nil {
			return err
		}

		currentPath = pathSoFar + "/" + name
		// log.Println("currentPath", currentPath)
		if paths != nil {
			*paths = append(*paths, currentPath)
		}
	}

	if download {
		if protonDrive.Config.DataFolderName == "" {
			return ErrDataFolderNameIsEmpty
		}

		if link.Type == proton.LinkTypeFile {
			log.Println("Downloading", currentPath)
			defer log.Println("Completes downloading", currentPath)

			reader, _, _, err := protonDrive.DownloadFile(ctx, link, 0)
			if err != nil {
				return err
			}

			byteArray, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			err = os.WriteFile("./"+protonDrive.Config.DataFolderName+"/"+currentPath, byteArray, 0777)
			if err != nil {
				return err
			}
		} else /* folder */ {
			if !(excludeRoot && curDepth == 0) {
				// log.Println("Creating folder", currentPath)
				// defer log.Println("Completes creating folder", currentPath)

				err := os.Mkdir("./"+protonDrive.Config.DataFolderName+"/"+currentPath, 0777)
				if err != nil {
					return err
				}
			}
		}
	}

	if maxDepth == -1 || curDepth < maxDepth {
		if link.Type == proton.LinkTypeFolder {
			childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, link.LinkID, true)
			if err != nil {
				return err
			}
			// log.Printf("childrenLinks len = %v, %#v", len(childrenLinks), childrenLinks)

			if childrenLinks != nil {
				// get current node's keyring
				signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.SignatureEmail})
				if err != nil {
					return err
				}
				linkKR, err := link.GetKeyRing(parentNodeKR, signatureVerificationKR)
				if err != nil {
					return err
				}

				for _, childLink := range childrenLinks {
					err = protonDrive.listDirectoriesRecursively(ctx, linkKR, &childLink, download, maxDepth, curDepth+1, excludeRoot, currentPath, paths)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
