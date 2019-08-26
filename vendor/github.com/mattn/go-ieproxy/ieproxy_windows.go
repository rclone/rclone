package ieproxy

import (
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

type regeditValues struct {
	ProxyServer   string
	ProxyOverride string
	ProxyEnable   uint64
	AutoConfigURL string
}

var once sync.Once
var windowsProxyConf ProxyConf

// GetConf retrieves the proxy configuration from the Windows Regedit
func getConf() ProxyConf {
	once.Do(writeConf)
	return windowsProxyConf
}

func writeConf() {
	var (
		cfg *tWINHTTP_CURRENT_USER_IE_PROXY_CONFIG
		err error
	)

	if cfg, err = getUserConfigFromWindowsSyscall(); err != nil {
		regedit, _ := readRegedit() // If the syscall fails, backup to manual detection.
		windowsProxyConf = parseRegedit(regedit)
		return
	}

	defer globalFreeWrapper(cfg.lpszProxy)
	defer globalFreeWrapper(cfg.lpszProxyBypass)
	defer globalFreeWrapper(cfg.lpszAutoConfigUrl)

	windowsProxyConf = ProxyConf{
		Static: StaticProxyConf{
			Active: cfg.lpszProxy != nil,
		},
		Automatic: ProxyScriptConf{
			Active: cfg.lpszAutoConfigUrl != nil || cfg.fAutoDetect,
		},
	}

	if windowsProxyConf.Static.Active {
		protocol := make(map[string]string)
		for _, s := range strings.Split(StringFromUTF16Ptr(cfg.lpszProxy), ";") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			pair := strings.SplitN(s, "=", 2)
			if len(pair) > 1 {
				protocol[pair[0]] = pair[1]
			} else {
				protocol[""] = pair[0]
			}
		}

		windowsProxyConf.Static.Protocols = protocol
		if cfg.lpszProxyBypass != nil {
			windowsProxyConf.Static.NoProxy = strings.Replace(StringFromUTF16Ptr(cfg.lpszProxyBypass), ";", ",", -1)
		}
	}

	if windowsProxyConf.Automatic.Active {
		windowsProxyConf.Automatic.PreConfiguredURL = StringFromUTF16Ptr(cfg.lpszAutoConfigUrl)
	}
}

func getUserConfigFromWindowsSyscall() (*tWINHTTP_CURRENT_USER_IE_PROXY_CONFIG, error) {
	handle, _, err := winHttpOpen.Call(0, 0, 0, 0, 0)
	if handle == 0 {
		return &tWINHTTP_CURRENT_USER_IE_PROXY_CONFIG{}, err
	}
	defer winHttpCloseHandle.Call(handle)

	config := new(tWINHTTP_CURRENT_USER_IE_PROXY_CONFIG)

	ret, _, err := winHttpGetIEProxyConfigForCurrentUser.Call(uintptr(unsafe.Pointer(config)))
	if ret > 0 {
		err = nil
	}

	return config, err
}

// OverrideEnvWithStaticProxy writes new values to the
// http_proxy, https_proxy and no_proxy environment variables.
// The values are taken from the Windows Regedit (should be called in init() function)
func overrideEnvWithStaticProxy(conf ProxyConf, setenv envSetter) {
	if conf.Static.Active {
		for _, scheme := range []string{"http", "https"} {
			url := mapFallback(scheme, "", conf.Static.Protocols)
			setenv(scheme+"_proxy", url)
		}
		if conf.Static.NoProxy != "" {
			setenv("no_proxy", conf.Static.NoProxy)
		}
	}
}

func parseRegedit(regedit regeditValues) ProxyConf {
	protocol := make(map[string]string)
	for _, s := range strings.Split(regedit.ProxyServer, ";") {
		if s == "" {
			continue
		}
		pair := strings.SplitN(s, "=", 2)
		if len(pair) > 1 {
			protocol[pair[0]] = pair[1]
		} else {
			protocol[""] = pair[0]
		}
	}

	return ProxyConf{
		Static: StaticProxyConf{
			Active:    regedit.ProxyEnable > 0,
			Protocols: protocol,
			NoProxy:   strings.Replace(regedit.ProxyOverride, ";", ",", -1), // to match linux style
		},
		Automatic: ProxyScriptConf{
			Active:           regedit.AutoConfigURL != "",
			PreConfiguredURL: regedit.AutoConfigURL,
		},
	}
}

func readRegedit() (values regeditValues, err error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	values.ProxyServer, _, err = k.GetStringValue("ProxyServer")
	if err != nil && err != registry.ErrNotExist {
		return
	}
	values.ProxyOverride, _, err = k.GetStringValue("ProxyOverride")
	if err != nil && err != registry.ErrNotExist {
		return
	}

	values.ProxyEnable, _, err = k.GetIntegerValue("ProxyEnable")
	if err != nil && err != registry.ErrNotExist {
		return
	}

	values.AutoConfigURL, _, err = k.GetStringValue("AutoConfigURL")
	if err != nil && err != registry.ErrNotExist {
		return
	}
	err = nil
	return
}
