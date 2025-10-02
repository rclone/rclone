package cluster

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
)

// Batches flow from queue/pending to queue/processing/
const (
	clusterQueue      = "queue"
	clusterPending    = clusterQueue + "/pending"
	clusterProcessing = clusterQueue + "/processing"
	clusterDone       = clusterQueue + "/done"
	clusterFinished   = clusterQueue + "/finished"
	clusterStatus     = clusterQueue + "/status"

	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential

	// Read the queue this often
	clusterCheckJobsInterval = time.Second

	// Write the worker status this often
	clusterWriteStatusInterval = time.Second

	// Read the worker status this often
	clusterCheckWorkersInterval = time.Second

	// Name of job which signals to the workers to quit
	quitJob = "QUIT"
)

// Jobs is a container for sending and receiving jobs to the cluster.
type Jobs struct {
	remote  string            // remote for job storage
	f       fs.Fs             // cluster remote storage
	partial bool              // do we need to write and rename
	hasMove bool              // set if f has server side move otherwise has server side copy
	cleanup fs.ClusterCleanup // how we cleanup the cluster files
	pacer   *fs.Pacer         // To pace the API calls
}

// NewJobs creates a Jobs source from the config in ctx.
//
// It may return nil for no cluster is configured.
func NewJobs(ctx context.Context) (*Jobs, error) {
	ci := fs.GetConfig(ctx)
	if ci.Cluster == "" {
		return nil, nil
	}
	f, err := cache.Get(ctx, ci.Cluster)
	if err != nil {
		return nil, fmt.Errorf("cluster remote creation: %w", err)
	}
	features := f.Features()
	if features.Move == nil && features.Copy == nil {
		return nil, fmt.Errorf("cluster remote must have server side move and %q doesn't", ci.Cluster)
	}
	jobs := &Jobs{
		remote:  ci.Cluster,
		f:       f,
		partial: features.PartialUploads,
		hasMove: features.Move != nil,
		pacer:   fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		cleanup: ci.ClusterCleanup,
	}
	return jobs, nil
}

// Create the cluster directory structure
func (jobs *Jobs) createDirectoryStructure(ctx context.Context) (err error) {
	for _, dir := range []string{clusterPending, clusterProcessing, clusterDone, clusterFinished, clusterStatus} {
		err = jobs.f.Mkdir(ctx, dir)
		if err != nil {
			return fmt.Errorf("cluster mkdir %q: %w", dir, err)
		}
	}
	return nil
}

// rename a file
//
// if this returns fs.ErrorObjectNotFound then the file has already been renamed.
func (jobs *Jobs) rename(ctx context.Context, src fs.Object, dstRemote string) (dst fs.Object, err error) {
	features := jobs.f.Features()
	if jobs.hasMove {
		dst, err = features.Move(ctx, src, dstRemote)
		if err != nil {
			return nil, fmt.Errorf("cluster: failed to rename job file: %w", err)
		}
	} else {
		dst, err = features.Copy(ctx, src, dstRemote)
		if err != nil {
			return nil, fmt.Errorf("cluster: failed to rename (copy phase) job file: %w", err)
		}
		err = src.Remove(ctx)
		if err != nil {
			return nil, fmt.Errorf("cluster: failed to rename (delete phase) job file: %w", err)
		}
	}
	return dst, nil
}

// Finish with a jobs file
func (jobs *Jobs) finish(ctx context.Context, obj fs.Object, status string, ok bool) {
	var err error
	if (ok && jobs.cleanup == fs.ClusterCleanupCompleted) || jobs.cleanup == fs.ClusterCleanupFull {
		err = obj.Remove(ctx)
	} else {
		name := path.Join(clusterFinished, status, path.Base(obj.Remote()))
		_, err = jobs.rename(ctx, obj, name)
	}
	if err != nil {
		fs.Errorf(nil, "cluster: removing completed job failed: %v", err)
	}
}

// write buf into remote
func (jobs *Jobs) writeFile(ctx context.Context, remote string, modTime time.Time, buf []byte) error {
	partialRemote := remote
	if jobs.partial {
		partialRemote = remote + ".partial"
	}
	// Calculate hashes
	w, err := hash.NewMultiHasherTypes(jobs.f.Hashes())
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	if err != nil {
		return err
	}
	obji := object.NewStaticObjectInfo(partialRemote, modTime, int64(len(buf)), true, w.Sums(), jobs.f)
	var obj fs.Object
	err = jobs.pacer.Call(func() (bool, error) {
		in := bytes.NewBuffer(buf)
		obj, err = jobs.f.Put(ctx, in, obji)
		if err != nil {
			return true, fmt.Errorf("cluster: failed to write %q: %q", remote, err)
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if jobs.partial {
		obj, err = jobs.rename(ctx, obj, remote)
		if err != nil {
			return err
		}
	}
	return nil
}

// Remove the file if it exists
func (jobs *Jobs) removeFile(ctx context.Context, remote string) error {
	obj, err := jobs.f.NewObject(ctx, remote)
	if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorDirNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	return obj.Remove(ctx)
}

// write a job to a file returning the name
func (jobs *Jobs) writeJob(ctx context.Context, where string, job any) (name string, err error) {
	now := time.Now().UTC()
	name = fmt.Sprintf("%s-%s", now.Format(time.RFC3339Nano), random.String(20))
	remote := path.Join(where, name+".json")
	buf, err := json.MarshalIndent(job, "", "\t")
	if err != nil {
		return "", fmt.Errorf("cluster: job json: %w", err)
	}
	err = jobs.writeFile(ctx, remote, now, buf)
	if err != nil {
		return "", fmt.Errorf("cluster: job write: %w", err)
	}
	return name, nil
}

// write a quit job to a file
func (jobs *Jobs) writeQuitJob(ctx context.Context, where string) (err error) {
	now := time.Now().UTC()
	remote := path.Join(where, quitJob+".json")
	err = jobs.writeFile(ctx, remote, now, []byte("{}"))
	if err != nil {
		return fmt.Errorf("cluster: quit job write: %w", err)
	}
	return nil
}

// read buf from object
func (jobs *Jobs) readFile(ctx context.Context, o fs.Object) (buf []byte, err error) {
	err = jobs.pacer.Call(func() (bool, error) {
		in, err := operations.Open(ctx, o)
		if err != nil {
			return true, fmt.Errorf("cluster: failed to open %q: %w", o, err)
		}
		buf, err = io.ReadAll(in)
		if err != nil {
			return true, fmt.Errorf("cluster: failed to read %q: %w", o, err)
		}
		err = in.Close()
		if err != nil {
			return true, fmt.Errorf("cluster: failed to close %q: %w", o, err)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// read a job from a file
//
// job should be a pointer to something to be unmarshalled
func (jobs *Jobs) readJob(ctx context.Context, obj fs.Object, job any) error {
	buf, err := jobs.readFile(ctx, obj)
	if err != nil {
		return fmt.Errorf("cluster: job read: %w", err)
	}
	err = json.Unmarshal(buf, job)
	if err != nil {
		return fmt.Errorf("cluster: job read json: %w", err)
	}
	return nil
}

// lists the json files in a cluster directory
func (jobs *Jobs) listDir(ctx context.Context, dir string) (objects []fs.Object, err error) {
	entries, err := jobs.f.List(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("cluster: failed to list %q: %w", dir, err)
	}
	entries.ForObject(func(o fs.Object) {
		if strings.HasSuffix(o.Remote(), ".json") {
			objects = append(objects, o)
		}
	})
	slices.SortStableFunc(objects, func(a, b fs.Object) int {
		return cmp.Compare(a.Remote(), b.Remote())
	})
	return objects, nil
}

// get a job from pending if there is one available.
//
// Returns a nil object if no jobs are available.
//
// FIXME should mark jobs as error jobs in here if they can't be read properly?
func (jobs *Jobs) getJob(ctx context.Context, id string) (name string, obj fs.Object, err error) {
	objs, err := jobs.listDir(ctx, clusterPending)
	if err != nil {
		return "", nil, fmt.Errorf("get job list: %w", err)
	}
	quit := false
	for len(objs) > 0 {
		obj = objs[0]
		objs = objs[1:]
		name = path.Base(obj.Remote())
		name, _ = strings.CutSuffix(name, ".json")

		// See if we have been asked to quit
		if name == quitJob {
			quit = true
			continue
		}

		// claim the job
		newName := fmt.Sprintf("%s-%s.json", name, id)
		newRemote := path.Join(clusterProcessing, newName)
		obj, err = jobs.rename(ctx, obj, newRemote)
		if errors.Is(err, fs.ErrorObjectNotFound) {
			// claim failed - try again
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("get job claim: %w", err)
		}
		return name, obj, nil
	}
	// No jobs found
	if quit {
		fs.Logf(nil, "Exiting cluster worker on command")
		atexit.Run()
		os.Exit(0)
	}
	return "", nil, nil
}
