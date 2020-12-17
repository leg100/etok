module github.com/leg100/etok

go 1.14

require (
	cloud.google.com/go v0.53.0 // indirect
	cloud.google.com/go/storage v1.5.0
	github.com/Sirupsen/logrus v0.0.0-00010101000000-000000000000 // indirect
	github.com/creack/pty v1.1.9
	github.com/docker/docker v1.13.1 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/fatih/color v1.7.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/google/go-cmp v0.5.2
	github.com/google/goexpect v0.0.0-20200816234442-b5b77125c2c5
	github.com/hashicorp/terraform-config-inspect v0.0.0-20201102131242-0c45ba392e51
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.5.1 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/sirupsen/logrus v1.5.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.14.1 // indirect
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20200831180312-196b9ba8737a // indirect
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0 // indirect
	golang.org/x/tools v0.0.0-20200403190813-44a64ad78b9b // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.0.0
	k8s.io/kubectl v0.18.6
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/client-go => k8s.io/client-go v0.18.6

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.4.2 // Suppresses https://github.com/sirupsen/logrus/issues/1041

replace google.golang.org/grpc => google.golang.org/grpc v1.29.0

//replace github.com/openshift/api => github.com/openshift/api v0.0.0-20190924102528-32369d4db2ad // Required until https://github.com/operator-framework/operator-lifecycle-manager/pull/1241 is resolved

replace github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible // https://github.com/kubernetes/client-go/issues/628
