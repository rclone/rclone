package drive

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"google.golang.org/api/driveactivity/v2"
)

const (
	initialBuffer = 10

	MoveQuery   = "detail.action_detail_case:MOVE"
	GlobalQuery = "time >= \"%s\" AND time < \"%s\""
)

type (
	parent struct {
		ID   string
		Name string
	}

	entryType struct {
		path      string
		entryType fs.EntryType
	}
)

var (
	consolidationStrategy = driveactivity.ConsolidationStrategy{
		None: new(driveactivity.NoConsolidation),
	}
)

func parseItemID(itemName string) string {
	return strings.ReplaceAll(itemName, "items/", "")
}

func createGlobalQueryRequest(rootFolderId, filter, pageToken string) *driveactivity.
	QueryDriveActivityRequest {
	return &driveactivity.QueryDriveActivityRequest{
		PageSize:              10,
		AncestorName:          fmt.Sprintf("items/%s", rootFolderId),
		ConsolidationStrategy: &consolidationStrategy,
		Filter:                filter,
		PageToken:             pageToken,
	}
}

func createItemQueryRequest(itemID, filter, pageToken string) *driveactivity.
	QueryDriveActivityRequest {
	return &driveactivity.QueryDriveActivityRequest{
		PageSize:              1,
		ItemName:              fmt.Sprintf("items/%s", itemID),
		ConsolidationStrategy: &consolidationStrategy,
		Filter:                filter,
		PageToken:             pageToken,
	}
}

func addParent(parents map[string]string, parent *parent) map[string]string {
	if _, ok := parents[parent.ID]; !ok {
		parents[parent.ID] = parent.Name
	}
	return parents
}

// Returns the type of a target and an associated title.
func parseTarget(target *driveactivity.Target) *driveactivity.DriveItem {
	return target.DriveItem
}

// Returns the first target for a list of targets.
func getTargetInfo(targets []*driveactivity.Target) (targetInfo *driveactivity.DriveItem) {
	for _, target := range targets {
		if targetInfo = parseTarget(target); targetInfo != nil {
			return
		}
	}
	return nil
}

func (f *Fs) getParent(target *driveactivity.DriveItem) *parent {
	targetTitle := target.Title
	q := createItemQueryRequest(parseItemID(target.Name), MoveQuery, "")
	response, err := f.activitySvc.Activity.Query(q).Do()
	if err != nil {
		fs.Errorf(targetTitle, "Unable to retrieve list of moves: %v", err)
	}
	fs.Debugf(targetTitle, "Retrieved Moves : %d\n", len(response.Activities))

	for _, activity := range response.Activities {
		for _, action := range activity.Actions {
			if action.Detail.Move != nil {
				for _, addedParent := range action.Detail.Move.AddedParents {
					parentItem := addedParent.DriveItem
					return &parent{ID: parseItemID(parentItem.Name), Name: parentItem.Title}
				}
			}
		}
	}
	return nil
}

func (f *Fs) parseActivityActions(driveTarget *driveactivity.DriveItem,
	actions []*driveactivity.Action) (invalidatedParents map[string]string) {

	var (
		invalidatedParent *parent
	)
	invalidatedParents = make(map[string]string)
	for _, action := range actions {
		target := driveTarget
		if action.Target != nil && parseTarget(action.Target) != nil {
			target = parseTarget(action.Target)
		}
		targetTitle := target.Title
		if action.Detail.Delete != nil {
			fs.Debugf(targetTitle, "Deleted")
			invalidatedParent = f.getParent(target)
			invalidatedParents = addParent(invalidatedParents, invalidatedParent)
		}
		if action.Detail.Rename != nil {
			fs.Debugf(targetTitle, "Renamed")
			invalidatedParent = f.getParent(target)
			invalidatedParents = addParent(invalidatedParents, invalidatedParent)
		}
		if action.Detail.Restore != nil {
			fs.Debugf(targetTitle, "Restored")
			invalidatedParent = f.getParent(target)
			invalidatedParents = addParent(invalidatedParents, invalidatedParent)
		}
		/* TODO: Verify if this is needed
		if action.Detail.Edit != nil {
			fs.Debugf(targetTitle, "Edited")
			invalidatedParent = f.getParent(target)
			invalidatedParents = addParent(invalidatedParents, invalidatedParent)
		}
		*/
		/* TODO: Verify if this is needed
		if action.Detail.PermissionChange != nil {
			fs.Debugf(targetTitle, "Permissions Changed")
			invalidatedParent = f.getParent(target)
			invalidatedParents = addParent(invalidatedParents, invalidatedParent)
		}
		*/
		if action.Detail.Move != nil {
			fs.Debugf(targetTitle, "Moved")
			moveAction := action.Detail.Move
			for _, addedParent := range moveAction.AddedParents {
				if addedParent.DriveItem == nil {
					fs.Errorf(targetTitle, "Invalid Parent: %v\n", addedParent)
				}
				parentItem := addedParent.DriveItem
				invalidatedParent = &parent{
					ID: parseItemID(parentItem.Name), Name: parentItem.Title}
				invalidatedParents = addParent(invalidatedParents, invalidatedParent)
			}
			for _, removedParent := range moveAction.RemovedParents {
				if removedParent.DriveItem == nil {
					fs.Errorf(targetTitle, "Invalid Parent: %v\n", removedParent)
				}
				parentItem := removedParent.DriveItem
				invalidatedParent = &parent{
					ID: parseItemID(parentItem.Name), Name: parentItem.Title}
				invalidatedParents = addParent(invalidatedParents, invalidatedParent)
			}
		}
		for _, invalidatedParent := range invalidatedParents {
			fs.Debugf(targetTitle, "Invalidated Parent : %s", invalidatedParent)
		}
	}
	return
}

func (f *Fs) parseActivity(activity *driveactivity.DriveActivity) map[string]string {
	target := getTargetInfo(activity.Targets)
	return f.parseActivityActions(target, activity.Actions)
}

func (f *Fs) changeNotifyRunner(ctx context.Context, fromTime,
	toTime time.Time, notifyFunc func(string, fs.EntryType)) {

	var (
		err                error
		pageToken          string
		pathsToClear       []entryType
		invalidatedParents map[string]string
		activityQuery      *driveactivity.QueryDriveActivityRequest
		activityResponse   *driveactivity.QueryDriveActivityResponse
	)

	filter := fmt.Sprintf(GlobalQuery,
		fromTime.Format(time.RFC3339), toTime.Format(time.RFC3339))

	for {
		activityQuery = createGlobalQueryRequest(f.rootFolderID, filter, pageToken)

		err = f.pacer.Call(func() (bool, error) {
			activityResponse, err = f.activitySvc.Activity.Query(activityQuery).Context(ctx).Do()
			return f.shouldRetry(err)
		})
		if err != nil {
			fs.Errorf(f, "Unable to retrieve list of activities: %v", err)
			return
		}
		fs.Infof(f, "Retrieved Activities : %d\n", len(activityResponse.Activities))

		for _, activity := range activityResponse.Activities {
			invalidatedParents = f.parseActivity(activity)

			for parentID, parentName := range invalidatedParents {
				parentName = f.opt.Enc.ToStandardName(parentName)
				// translate the path of this dir
				if parentPath, ok := f.dirCache.GetInv(parentID); ok {
					pathsToClear = append(pathsToClear, entryType{
						path: parentPath, entryType: fs.EntryDirectory})
				}
			}
		}

		visitedPaths := make(map[string]struct{})
		for _, entry := range pathsToClear {
			if _, ok := visitedPaths[entry.path]; ok {
				continue
			}
			visitedPaths[entry.path] = struct{}{}
			fs.Debugf(f, "Clearing Parent : %s (%v)", entry.path, entry.entryType)
			notifyFunc(entry.path, entry.entryType)
		}

		pageToken = activityResponse.NextPageToken
		if pageToken == "" {
			break
		}
	}
}
