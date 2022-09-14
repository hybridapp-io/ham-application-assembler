module github.com/hybridapp-io/ham-application-assembler

go 1.17

require (
	github.com/hybridapp-io/ham-deployable-operator v0.0.0-20210414204046-f5387dd09f68
	github.com/hybridapp-io/ham-placement v0.0.0-20210225195735-3057d58bb101
	github.com/kubernetes-sigs/application v0.8.1
	github.com/onsi/gomega v1.10.5
	github.com/open-cluster-management/api v0.0.0-20210527013639-a6845f2ebcb1
	github.com/open-cluster-management/multicloud-operators-deployable v0.0.0-20200721140654-267157672e39
	github.com/open-cluster-management/multicloud-operators-subscription v1.0.0-2020-05-12-21-17-19.0.20200721224621-79fd9d450d82
	github.com/operator-framework/operator-sdk v0.18.0
	github.com/prometheus/common v0.10.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.11
	k8s.io/apiextensions-apiserver v0.20.11
	k8s.io/apimachinery v0.20.11
	k8s.io/client-go v13.0.0+incompatible
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.6.5
)

require github.com/go-logr/logr v0.2.1 // indirect

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v14.2.0+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.20.11
)
