#!/bin/sh
# This tests rclone compiles for all the commits in the branch
#
# It assumes that the branch is rebased onto master and checks all the commits from branch root to master
#
# Adapted from: https://blog.ploeh.dk/2013/10/07/verifying-every-single-commit-in-a-git-branch/

BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" = "master" ]; then
    echo "Don't run on master branch"
    exit 1
fi
COMMITS=$(git log --oneline --reverse master.. | cut -d " " -f 1)
CODE=0

for COMMIT in $COMMITS
do
    git checkout $COMMIT
    
    # run-tests
    echo "------------------------------------------------------------"
    go install ./...
    
    if [ $? -eq 0 ]
    then
        echo $COMMIT - passed
    else
        echo $COMMIT - failed
        git checkout ${BRANCH}
        exit
    fi
    echo "------------------------------------------------------------"
done
 
git checkout ${BRANCH}
echo "All OK"
