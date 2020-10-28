module github.com/leg100/stok

go 1.14

require (
	cloud.google.com/go v0.53.0 // indirect
	cloud.google.com/go/storage v1.5.0
	github.com/Sirupsen/logrus v0.0.0-00010101000000-000000000000 // indirect
	github.com/creasty/defaults v1.5.1
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/fatih/color v1.7.0
	github.com/go-logr/logr v0.1.0
	github.com/google/go-cmp v0.4.0
	github.com/kr/pretty v0.2.0 // indirect
	github.com/kr/pty v1.1.5
	github.com/operator-framework/operator-sdk v0.19.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/tj/assert v0.0.3
	golang.org/x/crypto v0.0.0-20200422194213-44a606286825
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200831180312-196b9ba8737a
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kubectl v0.18.2
	k8s.io/utils v0.0.0-20200327001022-6496210b90e8 // indirect
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/client-go => k8s.io/client-go v0.18.2

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.4.2 // Suppresses https://github.com/sirupsen/logrus/issues/1041

//replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible // https://github.com/kubernetes/client-go/issues/628
