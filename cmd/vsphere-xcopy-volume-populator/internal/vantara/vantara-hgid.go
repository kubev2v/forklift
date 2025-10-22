package vantara

import (
	"strings"

	"k8s.io/klog/v2"
)

type Logins struct {
	HostGroupId     string `json:"hostGroupId"`
	Islogin         string `json:"isLogin"`
	LoginWWN        string `json:"loginWwn"`
	WWNNickName     string `json:"wwnNickName"`
	IscsiNickName   string `json:"iscsiNickName"`
	IscsiTargetName string `json:"iscsiTargetName"`
	LoginIscsiName  string `json:"loginIscsiName"`
}

type DataEntry struct {
	PortID string   `json:"portId"`
	WWN    string   `json:"wwn"`
	Logins []Logins `json:"logins"`
}

type JSONData struct {
	Data []DataEntry `json:"data"`
}

func FindHostGroupIDs(jsonData JSONData, hbaUIDs []string) []Logins {
	results := []Logins{}
	for _, entry := range jsonData.Data {
		for _, login := range entry.Logins {
			for _, uid := range hbaUIDs {
				if strings.HasPrefix(uid, "fc.") {
					parts := strings.Split(strings.TrimPrefix(uid, "fc."), ":")
					wwnn := ""
					if len(parts) != 2 {
						klog.Errorf("Invalid FC WWN: %s", uid)
						continue
					} else {
						wwnn = strings.ToUpper(parts[1])
					}
					if login.LoginWWN == wwnn {
						output := Logins{
							HostGroupId:     login.HostGroupId,
							Islogin:         login.Islogin,
							LoginWWN:        login.LoginWWN,
							WWNNickName:     login.WWNNickName,
							IscsiNickName:   "",
							IscsiTargetName: "",
							LoginIscsiName:  "",
						}
						results = append(results, output)
					}
				} else if strings.HasPrefix(uid, "iqn.") {
					continue
				} else if strings.HasPrefix(uid, "nqn.") {
					continue
				} else {
					klog.Errorf("Unknown UID type: %s", uid)
				}
			}

		}
	}
	return results
}
