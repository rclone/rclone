package fuse

func localVolume(conf *mountConfig) error {
	return nil
}

func volumeName(name string) MountOption {
	return dummyOption
}

func daemonTimeout(name string) MountOption {
	return dummyOption
}

func noAppleXattr(conf *mountConfig) error {
	return nil
}

func noAppleDouble(conf *mountConfig) error {
	return nil
}

func exclCreate(conf *mountConfig) error {
	return nil
}

func noBrowse(conf *mountConfig) error {
	return nil
}

func maxPages(count uint16) MountOption {
	return func(conf *mountConfig) error {
		if count < 32 {
			count = 32 // The minimum allowed by the kernel
		} else if count > 256 {
			count = 256 // The maximum allowed by the kernel
		}
		conf.maxPages = count
		return nil
	}
}
