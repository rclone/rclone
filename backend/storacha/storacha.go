package storacha

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/guppy/pkg/client"
)

//space -> fs

type Fs struct {
	spaceDID did.DID
	cli      *client.Client
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "storacha",
		Description: "Storacha via guppy",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "email",
			Help:     "Email address to use for Storacha authentication.",
			Required: false,
			Default:  "",
		}, {
			Name:     "private_key_path",
			Help:     "Path to the base64-encoded private key file for Storacha authentication.",
			Required: false,
			Default:  "",
		}, {
			Name:     "proof_path",
			Help:     "Path to the base64-encoded proof file for Storacha authentication.",
			Required: false,
			Default:  "",
		},
			{
				Name:     "space_did",
				Help:     "DID of the Storacha space to access.",
				Required: false,
				Default:  "",
			},
		},
	})
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	email, hasEmail := m.Get("email")
	privateKeyPath, hasPK := m.Get("private_key_path")
	proofPath, hasProof := m.Get("proof_path")
	spaceDID, hasSpace := m.Get("space_did")

	var cli *client.Client
	var err error

	switch {
	case hasEmail:
		cli, err = EmailAuth(email)
		if err != nil {
			return nil, fmt.Errorf("email auth failed: %w", err)
		}

	case hasPK || hasProof || hasSpace:
		if !hasPK || !hasProof || !hasSpace {
			return nil, fmt.Errorf(
				"incomplete private key auth config: require private_key_path, proof_path, space_did",
			)
		}

		authConfig := AuthConfig{
			PrivateKeyPath: privateKeyPath,
			ProofPath:      proofPath,
			SpaceDID:       spaceDID,
		}

		if err := ValidateAuthConfig(&authConfig); err != nil {
			return nil, fmt.Errorf("invalid Storacha auth config: %w", err)
		}

		cli, err = PrivateKeyAuth(&authConfig)
		if err != nil {
			return nil, fmt.Errorf("private key auth failed: %w", err)
		}

	default:
		return nil, fmt.Errorf(
			"no authentication method provided: specify either email or private key credentials",
		)
	}
	if !hasSpace {
		return nil, fmt.Errorf("space_did is required to create Storacha filesystem")
	}

	parsedDID, err := did.Parse(spaceDID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse space_did: %w", err)
	}

	fsys, err := NewStorachaFilesystem(cli, parsedDID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Storacha filesystem: %w", err)
	}

	return fsys, nil
}

func NewStorachaFilesystem(cli *client.Client, spaceDID did.DID) (fs.Fs, error) {
	return &Fs{
		spaceDID: spaceDID,
		cli:      cli,
	}, nil
}

// Minimal fs.Fs interface implementation
func (f *Fs) Name() string {
	return "storacha"
}

func (f *Fs) Root() string {
	return ""
}

func (f *Fs) Features() *fs.Features {
	return nil
}

func (f *Fs) String() string {
	return fmt.Sprintf("Storacha FS (space: %s)", f.spaceDID.String())
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	var cursor *string
	for {
		listOk, err := f.cli.UploadList(
			ctx,
			f.spaceDID,
			uploadcap.ListCaveats{Cursor: cursor})

		if err != nil {
			return nil, fmt.Errorf("failed to list uploads: %w", err)
		}

		for _, r := range listOk.Results {
			fmt.Printf("%s\n", r.Root)
			if lsFlags.showShards {
				for _, s := range r.Shards {
					fmt.Printf("\t%s\n", s)
				}
			}
		}

		if listOk.Cursor == nil {
			break
		}
		cursor = listOk.Cursor
	}
	return nil, nil
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// 3 cases possible:
	// the current is a space 
	// so creating a new directory basically means making a new upload in the space 
	// if there is mid directory we need to change the cid so we need to create a new upload with the new cid
}


