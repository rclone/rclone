---
title: "Cluster"
description: "Clustering rclone"
versionIntroduced: "v1.72"
---

# Cluster

Rclone has a cluster mode invoked with the `--cluster` flag. This
enables a group of rclone instances to work together on doing a sync.

This is controlled by a group of flags starting with `--cluster-` and
enabled with the `--cluster` flag.

```text
--cluster string                   Enable cluster mode with remote to use as shared storage
--cluster-batch-files int          Max number of files for a cluster batch (default 1000)
--cluster-batch-size SizeSuffix    Max size of files for a cluster batch (default 1Ti)
--cluster-cleanup ClusterCleanup   Control which cluster files get cleaned up (default full)
--cluster-id string                Set to an ID for the cluster. An ID of 0 or empty becomes the controller
--cluster-quit-workers             Set to cause the controller to quit the workers when it finished
```

The command might look something like this which is a normal rclone
command but with a new `--cluster` flag which points at an rclone
remote defining the cluster storage. This is the signal to rclone that
it should engage the cluster mode with a controller and workers.

```sh
rclone copy source: destination: --flags --cluster /work
rclone copy source: destination: --flags --cluster s3:bucket
```

This works only with the `rclone sync`, `copy` and `move` commands.

If the remote specified by the `--cluster` command is inside the
`source:` or `destination:` it must be excluded with the filter flags.

Any rclone remotes used in the transfer must be defined in all cluster
nodes. Defining remotes with connection strings will get around that
problem.

## Terminology

The cluster has two logical groups, the controller and the workers.
There is one controller and many workers.

The controller and the workers will communicate with each other by
creating files in the remote pointed to by the `--cluster` flag. This
could be for example an S3 bucket or a Kubernetes PVC.

The files are JSON serialized rc commands. Multiple commands are sent
using `rc/batch`. The commands flow `pending` →`processing` → `done` →
`finished`

```text
└── queue
    ├── pending    ← pending task files created by the controller
    ├── processing ← claimed tasks being executed by a worker
    ├── done       ← finished tasks awaiting the controller to read the result
    └── finished   ← completed task files
```

The cluster can be set up in two ways as a persistent cluster or as a
transient cluster.

### Persistent cluster

Run a cluster of workers using

```sh
rclone rcd --cluster /work
```

Then run rclone commands when required on the cluster:

```sh
rclone copy source: destination: --flags --cluster /work
```

In this mode there can be many rclone commands executing at once.

### Transient cluster

Run many copies of rclone simultaneously, for example in a Kubernetes
indexed job.

The rclone with `--cluster-id 0` becomes the controller and the others
become the workers. For a Kubernetes indexed job, setting
`--cluster-id $(JOB_COMPLETION_INDEX)` would work well.

Add the `--cluster-quit-workers` flag - this will cause the controller
to make sure the workers exit when it has finished.

All instances of rclone run a command like this so the whole cluster
can only run one rclone command:

```sh
rclone copy source: destination: --flags --cluster /work --cluster-id $(JOB_COMPLETION_INDEX) --cluster-quit-workers
```

## Controller

The controller runs the sync and work distribution.

- It does the listing of the source and destination directories
  comparing files in order to find files which need to be transferred.
- Files which need to be transferred are then batched into jobs of
  `--cluster-batch-files` files to transfer or `--cluster-batch-size`
  max size in `queue/pending` for the workers to pick up.
- It watches `queue/done` for finished jobs and updates the transfer
  statistics and logs any errors, accordingly moving the job to
  `queue/finished`.

Once the sync is complete, if `--cluster-quit-workers` is set, then it
sends the workers a special command which causes them all to exit.

The controller only sends transfer jobs to the workers. All the other
tasks (eg listing, comparing) are done by the controller. The
controller does not execute any transfer tasks itself.

## Workers

The workers job is entirely to act as API endpoints that receive their
work via files in `/work`. Then

- Read work in `queue/pending`
- Attempt to rename into `queue/processing`
- If the cluster work directory supports atomic renames, then use
  those, otherwise read the file, write the copy, delete the original.
  If the delete fails then the rename was not successful (possible on
  s3 backends).
- If successful then do that item of work. If not successful another
  worker got there first and sleep for a bit then retry.
- After the copy is complete then remove the `queue/processing` file
  or rename it into `queue/finished` if the `--cluster-cleanup` flag
  allows it.
- Repeat

## Flags

### --cluster string

This enables the cluster mode. Without this flag, all the other
cluster flags are ignored. This should be given a remote which can be
a local directory, eg `/work` or a remote directory, eg `s3:bucket`.

### --cluster-batch-files int

This controls the number of files copied in a cluster batch. Setting
this larger may be more efficient but it means the statistics will be
less accurate on the controller (default 1000).

### --cluster-batch-size SizeSuffix

This controls the total size of files in a cluster batch. If the size
of the files in a batch exceeds this number then the batch will be
sent to the workers. Setting this larger may be more efficient but it
means the statistics will be less accurate on the controller. (default
1TiB)

### --cluster-cleanup ClusterCleanup

Controls which cluster files get cleaned up.

- `full` - clean all work files (default)
- `completed` - clean completed work files but leave the errors and status
- `none` - leave all the file (useful for debugging)

### --cluster-id string

Set an ID for the rclone instance. This can be a string or a number.
An ID of 0 will become the controller otherwise the instance will
become a worker. If this flag isn't supplied or the value is empty,
then a random string will be used instead.

### --cluster-quit-workers

If this flag is set, then when the controller finishes its sync task
it will quit all the workers before it exits.

## Not implemented

Here are some features from the original design which are not
implemented yet:

- the controller will not notice if workers die or fail to complete
  their tasks
- the controller does not re-assign the workers work if necessary
- the controller does not restart the sync
- the workers do not write any status files (but the stats are
  correctly accounted)
