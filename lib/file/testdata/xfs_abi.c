/* Print the installed XFS commit-range UAPI for xfs_linux_test.go. */
#include <stddef.h>
#include <stdio.h>
#include <xfs/xfs.h>
#include <xfs/xfs_fs.h>

int main(void)
{
	printf("%zu %zu %zu %zu %zu %zu %zu %zu %lu %lu %llu %llu %zu %zu %lu %u\n",
		sizeof(struct xfs_commit_range),
		offsetof(struct xfs_commit_range, file1_fd),
		offsetof(struct xfs_commit_range, pad),
		offsetof(struct xfs_commit_range, file1_offset),
		offsetof(struct xfs_commit_range, file2_offset),
		offsetof(struct xfs_commit_range, length),
		offsetof(struct xfs_commit_range, flags),
		offsetof(struct xfs_commit_range, file2_freshness),
		(unsigned long)XFS_IOC_START_COMMIT,
		(unsigned long)XFS_IOC_COMMIT_RANGE,
		(unsigned long long)XFS_EXCHANGE_RANGE_TO_EOF,
		(unsigned long long)XFS_EXCHANGE_RANGE_DSYNC,
		sizeof(struct xfs_fsop_geom), offsetof(struct xfs_fsop_geom, flags),
		(unsigned long)XFS_IOC_FSGEOMETRY, (unsigned)XFS_FSOP_GEOM_FLAGS_EXCHANGE_RANGE);
	return 0;
}
