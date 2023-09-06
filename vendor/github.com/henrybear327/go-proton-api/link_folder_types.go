package proton

import (
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type CreateFolderReq struct {
	ParentLinkID string

	Name string
	Hash string

	NodeKey     string
	NodeHashKey string

	NodePassphrase          string
	NodePassphraseSignature string

	SignatureAddress string
}

func (createFolderReq *CreateFolderReq) SetName(name string, addrKR, nodeKR *crypto.KeyRing) error {
	encNameString, err := getEncryptedName(name, addrKR, nodeKR)
	if err != nil {
		return err
	}

	createFolderReq.Name = encNameString

	return nil
}

func (createFolderReq *CreateFolderReq) SetHash(name string, hashKey []byte) error {
	nameHash, err := GetNameHash(name, hashKey)
	if err != nil {
		return err
	}

	createFolderReq.Hash = nameHash

	return nil
}

func (createFolderReq *CreateFolderReq) SetNodeHashKey(parentNodeKey *crypto.KeyRing) error {
	token, err := crypto.RandomToken(32)
	if err != nil {
		return err
	}

	tokenMessage := crypto.NewPlainMessage(token)

	encToken, err := parentNodeKey.Encrypt(tokenMessage, parentNodeKey)
	if err != nil {
		return err
	}

	nodeHashKey, err := encToken.GetArmored()
	if err != nil {
		return err
	}

	createFolderReq.NodeHashKey = nodeHashKey

	return nil
}

type CreateFolderRes struct {
	ID string // Encrypted Link ID
}
