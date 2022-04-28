#!/usr/bin/env bash
exec rclone --check-normalization=true --check-control=true --check-length=true info \
	/tmp/testInfo \
	TestAmazonCloudDrive:testInfo \
	TestB2:testInfo \
	TestCryptDrive:testInfo \
	TestCryptSwift:testInfo \
	TestDrive:testInfo \
	TestDropbox:testInfo \
	TestGoogleCloudStorage:rclone-testinfo \
	TestnStorage:testInfo \
	TestOneDrive:testInfo \
	TestS3:rclone-testinfo \
	TestSftp:testInfo \
	TestSwift:testInfo \
	TestYandex:testInfo \
	TestFTP:testInfo

#	TestHubic:testInfo \
