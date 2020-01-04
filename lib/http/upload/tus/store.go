package tus

import (
	"context"
	"errors"
	up "github.com/rclone/rclone/lib/http/upload"
	tus "github.com/tus/tusd/pkg/handler"
)

type fsStore struct {
	create up.CreateCallback
	get    up.GetCallback
}

// NewUpload calls the create callback and returns a upload object wrapping the returned fs.Object
func (s *fsStore) NewUpload(ctx context.Context, info tus.FileInfo) (tus.Upload, error) {
	object := s.create(ctx, info.Size, up.MetaData(info.MetaData))
	if object == nil {
		return nil, errors.New("failed to create object")
	}

	return &upload{object, &info}, nil
}

// GetUpload calls the get callback and returns a upload object wrapping the returned fs.Object
func (s *fsStore) GetUpload(ctx context.Context, id string) (tus.Upload, error) {
	object := s.get(ctx)
	if object == nil {
		return nil, errors.New("failed to get object")
	}

	return &upload{object, nil}, nil
}

func (s *fsStore) AsLengthDeclarableUpload(u tus.Upload) tus.LengthDeclarableUpload {
	return u.(*upload)
}

func (s *fsStore) AsTerminatableUpload(u tus.Upload) tus.TerminatableUpload {
	return u.(*upload)
}

func (s *fsStore) AsConcatableUpload(u tus.Upload) tus.ConcatableUpload {
	return u.(*upload)
}

func (s *fsStore) NewLock(id string) (tus.Lock, error) {
	u, err := s.GetUpload(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return u.(*upload).NewLock()
}

func (s *fsStore) UseIn(composer *tus.StoreComposer) {
	composer.UseCore(s)
	composer.UseConcater(s)
	composer.UseLengthDeferrer(s)
	composer.UseLocker(s)
	composer.UseTerminater(s)
}
