package logger

import "k8s.io/klog/v2"

const RootName = "copy-offload"

func New(providerName string) klog.Logger {
	return klog.Background().WithName(RootName).WithName(providerName)
}
