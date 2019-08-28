// +build !windows

package ieproxy

func getConf() ProxyConf {
	return ProxyConf{}
}

func overrideEnvWithStaticProxy(pc ProxyConf, setenv envSetter) {
}
