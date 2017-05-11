package fuse

// for TestMountOptionCommaError
func ForTestSetMountOption(k, v string) MountOption {
	fn := func(conf *mountConfig) error {
		conf.options[k] = v
		return nil
	}
	return fn
}
