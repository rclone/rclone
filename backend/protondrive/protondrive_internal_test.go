package protondrive

import "testing"

type configWithDriveSDKVersion struct {
	DriveSDKVersion string
}

type configWithoutDriveSDKVersion struct {
	AppVersion string
}

func TestSetDriveSDKVersionIfSupportedSetsField(t *testing.T) {
	config := &configWithDriveSDKVersion{}
	setDriveSDKVersionIfSupported(config, "js@0.10.0")

	if config.DriveSDKVersion != "js@0.10.0" {
		t.Fatalf("expected DriveSDKVersion to be set, got %q", config.DriveSDKVersion)
	}
}

func TestSetDriveSDKVersionIfSupportedIgnoresMissingField(t *testing.T) {
	config := &configWithoutDriveSDKVersion{AppVersion: "app"}
	setDriveSDKVersionIfSupported(config, "js@0.10.0")

	if config.AppVersion != "app" {
		t.Fatalf("unexpected mutation: %+v", config)
	}
}
