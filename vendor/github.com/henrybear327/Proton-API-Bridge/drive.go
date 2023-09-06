package proton_api_bridge

import (
	"context"
	"log"

	"github.com/henrybear327/Proton-API-Bridge/common"
	"golang.org/x/sync/semaphore"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type ProtonDrive struct {
	MainShare *proton.Share
	RootLink  *proton.Link

	MainShareKR *crypto.KeyRing
	AddrKR      *crypto.KeyRing

	Config *common.Config

	c                *proton.Client
	m                *proton.Manager
	userKR           *crypto.KeyRing
	addrKRs          map[string]*crypto.KeyRing
	addrData         []proton.Address
	signatureAddress string

	cache                *cache
	blockUploadSemaphore *semaphore.Weighted
	blockCryptoSemaphore *semaphore.Weighted
}

func NewDefaultConfig() *common.Config {
	return common.NewConfigWithDefaultValues()
}

func NewProtonDrive(ctx context.Context, config *common.Config, authHandler proton.AuthHandler, deAuthHandler proton.Handler) (*ProtonDrive, *common.ProtonDriveCredential, error) {
	/* Log in and logout */
	m, c, credentials, userKR, addrKRs, addrData, err := common.Login(ctx, config, authHandler, deAuthHandler)
	if err != nil {
		return nil, nil, err
	}

	/*
		Current understanding (at the time of the commit)

		The volume is the mount point.

		A link is like a folder in POSIX.

		A share is associated with a link to represent the access control,
		and serves as an entry point to a location in the file structure (Volume).
		It points to a link, of file or folder type, anywhere in the tree and holds a key called the ShareKey.
		To access a link, of file or folder type, a user must be a member of a share.

		A volume has a default share for access control and is owned by the creator of the volume.
		A volume has a default link as it's root folder.

		MIMETYPE holds type, e.g. folder, image/png, etc.
	*/
	volumes, err := listAllVolumes(ctx, c)
	if err != nil {
		return nil, nil, err
	}
	// log.Printf("all volumes %#v", volumes)

	mainShareID := ""
	for i := range volumes {
		// iOS drive: first active volume
		if volumes[i].State == proton.VolumeStateActive {
			mainShareID = volumes[i].Share.ShareID
		}
	}
	// log.Println("total volumes", len(volumes), "mainShareID", mainShareID)

	/* Get root folder from the main share of the volume */
	mainShare, err := getShareByID(ctx, c, mainShareID)
	if err != nil {
		return nil, nil, err
	}

	// check for main share integrity
	{
		mainShareCheck := false
		shares, err := getAllShares(ctx, c)
		if err != nil {
			return nil, nil, err
		}
		for i := range shares {
			if shares[i].ShareID == mainShare.ShareID &&
				shares[i].LinkID == mainShare.LinkID &&
				shares[i].Flags == proton.PrimaryShare &&
				shares[i].Type == proton.ShareTypeMain {
				mainShareCheck = true
			}
		}

		if !mainShareCheck {
			log.Printf("mainShare %#v", mainShare)
			log.Printf("shares %#v", shares)
			return nil, nil, ErrMainSharePreconditionsFailed
		}
	}

	// Note: rootLink's parentLinkID == ""
	/*
		Link holds the tree structure, for the clients, they represent the files and folders of a given volume.
		They have a ParentLinkID that points to parent folders.
		Links also hold the file name (encrypted) and a hash of the name for name collisions.
		Link data is encrypted with its owning Share keyring.
	*/
	rootLink, err := c.GetLink(ctx, mainShare.ShareID, mainShare.LinkID)
	if err != nil {
		return nil, nil, err
	}
	if err != nil {
		return nil, nil, err
	}
	// log.Printf("rootLink %#v", rootLink)

	// log.Printf("addrKRs %#v", addrKRs)=
	addrKR := addrKRs[mainShare.AddressID]
	// log.Println("addrKR CountDecryptionEntities", addrKR.CountDecryptionEntities())

	mainShareKR, err := mainShare.GetKeyRing(addrKR)
	if err != nil {
		return nil, nil, err
	}
	// log.Println("mainShareKR CountDecryptionEntities", mainShareKR.CountDecryptionEntities())

	return &ProtonDrive{
		MainShare: mainShare,
		RootLink:  &rootLink,

		MainShareKR: mainShareKR,
		AddrKR:      addrKR,

		Config: config,

		c:                c,
		m:                m,
		userKR:           userKR,
		addrKRs:          addrKRs,
		addrData:         addrData,
		signatureAddress: mainShare.Creator,

		cache:                newCache(config.EnableCaching),
		blockUploadSemaphore: semaphore.NewWeighted(int64(config.ConcurrentBlockUploadCount)),
		blockCryptoSemaphore: semaphore.NewWeighted(int64(config.ConcurrentFileCryptoCount)),
	}, credentials, nil
}

func (protonDrive *ProtonDrive) Logout(ctx context.Context) error {
	return common.Logout(ctx, protonDrive.Config, protonDrive.m, protonDrive.c, protonDrive.userKR, protonDrive.addrKRs)
}

func (protonDrive *ProtonDrive) About(ctx context.Context) (*proton.User, error) {
	user, err := protonDrive.c.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (protonDrive *ProtonDrive) GetLink(ctx context.Context, linkID string) (*proton.Link, error) {
	return protonDrive.getLink(ctx, linkID)
}
