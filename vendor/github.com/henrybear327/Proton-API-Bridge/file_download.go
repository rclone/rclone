package proton_api_bridge

import (
	"bytes"
	"context"
	"io"
	"log"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type FileDownloadReader struct {
	protonDrive *ProtonDrive
	ctx         context.Context

	link         *proton.Link
	data         *bytes.Buffer
	nodeKR       *crypto.KeyRing
	sessionKey   *crypto.SessionKey
	revision     *proton.Revision
	nextRevision int

	isEOF bool

	// TODO: integrity check if the entire file is read
}

func (r *FileDownloadReader) Read(p []byte) (int, error) {
	if r.data.Len() == 0 {
		// TODO: do we have memory sharing bug?
		// to avoid sharing the underlying buffer array across re-population
		r.data = bytes.NewBuffer(nil)

		// we download and decrypt more content
		err := r.populateBufferOnRead()
		if err != nil {
			return 0, err
		}

		if r.isEOF {
			// if the file has been downloaded entirely, we return EOF
			return 0, io.EOF
		}
	}

	return r.data.Read(p)
}

func (r *FileDownloadReader) Close() error {
	r.protonDrive = nil

	return nil
}

func (reader *FileDownloadReader) populateBufferOnRead() error {
	if len(reader.revision.Blocks) == 0 || len(reader.revision.Blocks) == reader.nextRevision {
		reader.isEOF = true
		return nil
	}

	offset := reader.nextRevision
	for i := offset; i-offset < DOWNLOAD_BATCH_BLOCK_SIZE && i < len(reader.revision.Blocks); i++ {
		// TODO: parallel download
		blockReader, err := reader.protonDrive.c.GetBlock(reader.ctx, reader.revision.Blocks[i].BareURL, reader.revision.Blocks[i].Token)
		if err != nil {
			return err
		}
		defer blockReader.Close()

		signatureVerificationKR, err := reader.protonDrive.getSignatureVerificationKeyring([]string{reader.link.SignatureEmail}, reader.nodeKR)
		if err != nil {
			return err
		}
		err = decryptBlockIntoBuffer(reader.sessionKey, signatureVerificationKR, reader.nodeKR, reader.revision.Blocks[i].Hash, reader.revision.Blocks[i].EncSignature, reader.data, blockReader)
		if err != nil {
			return err
		}

		reader.nextRevision = i + 1
	}

	return nil
}

func (protonDrive *ProtonDrive) DownloadFileByID(ctx context.Context, linkID string, offset int64) (io.ReadCloser, int64, *FileSystemAttrs, error) {
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(linkID, false)

	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, 0, nil, err
	}

	return protonDrive.DownloadFile(ctx, link, offset)
}

func (protonDrive *ProtonDrive) DownloadFile(ctx context.Context, link *proton.Link, offset int64) (io.ReadCloser, int64, *FileSystemAttrs, error) {
	if link.Type != proton.LinkTypeFile {
		return nil, 0, nil, ErrLinkTypeMustToBeFileType
	}

	parentNodeKR, err := protonDrive.getLinkKRByID(ctx, link.ParentLinkID)
	if err != nil {
		return nil, 0, nil, err
	}

	signatureVerificationKR, err := protonDrive.getSignatureVerificationKeyring([]string{link.SignatureEmail})
	if err != nil {
		return nil, 0, nil, err
	}
	nodeKR, err := link.GetKeyRing(parentNodeKR, signatureVerificationKR)
	if err != nil {
		return nil, 0, nil, err
	}

	sessionKey, err := link.GetSessionKey(nodeKR)
	if err != nil {
		return nil, 0, nil, err
	}

	revision, fileSystemAttrs, err := protonDrive.GetActiveRevisionWithAttrs(ctx, link)
	if err != nil {
		return nil, 0, nil, err
	}

	reader := &FileDownloadReader{
		protonDrive: protonDrive,
		ctx:         ctx,

		link:         link,
		data:         bytes.NewBuffer(nil),
		nodeKR:       nodeKR,
		sessionKey:   sessionKey,
		revision:     revision,
		nextRevision: 0,

		isEOF: false,
	}

	useFallbackDownload := false
	if fileSystemAttrs != nil {
		// based on offset, infer the nextRevision (0-based)
		if fileSystemAttrs.BlockSizes == nil {
			useFallbackDownload = true
		} else {
			// infer nextRevision
			totalBytes := int64(0)
			for i := 0; i < len(fileSystemAttrs.BlockSizes); i++ {
				prevTotalBytes := totalBytes
				totalBytes += fileSystemAttrs.BlockSizes[i]
				if offset <= totalBytes {
					offset = offset - prevTotalBytes
					reader.nextRevision = i
					break
				}
			}

			// download will start from the specified block
			n, err := io.CopyN(io.Discard, reader, offset)
			if err != nil {
				return nil, 0, nil, err
			}
			if int64(n) != offset {
				return nil, 0, nil, ErrSeekOffsetAfterSkippingBlocks
			}
		}
	}

	if useFallbackDownload {
		log.Println("Performing inefficient seek as metadata of encrypted file is missing")
		n, err := io.CopyN(io.Discard, reader, offset)
		if err != nil {
			return nil, 0, nil, err
		}
		if int64(n) != offset {
			return nil, 0, nil, ErrSeekOffsetAfterSkippingBlocks
		}
	}
	return reader, link.Size, fileSystemAttrs, nil
}
