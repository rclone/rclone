package fuse

func daemonTimeout(name string) MountOption {
	return func(conf *mountConfig) error {
		conf.options["timeout"] = name
		return nil
	}
}
