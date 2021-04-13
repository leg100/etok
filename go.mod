module github.com/leg100/etok

go 1.16

require (
	cloud.google.com/go/storage v1.12.0
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/aws/aws-sdk-go v1.37.3
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/creack/pty v1.1.11
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/fatih/color v1.10.0
	github.com/fsouza/fake-gcs-server v1.22.0
	github.com/go-git/go-git/v5 v5.3.0
	github.com/google/go-cmp v0.5.4
	github.com/google/go-github/v31 v31.0.0
	github.com/google/goexpect v0.0.0-20200816234442-b5b77125c2c5
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/hcl/v2 v2.6.0 // indirect
	github.com/hashicorp/terraform-config-inspect v0.0.0-20201102131242-0c45ba392e51
	github.com/johannesboyne/gofakes3 v0.0.0-20210124080349-901cf567bf01
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/unrolled/render v1.0.3
	github.com/urfave/negroni v1.0.0
	github.com/zclconf/go-cty v1.5.1 // indirect
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	google.golang.org/api v0.36.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/klog/v2 v2.8.0
	k8s.io/kubectl v0.19.2
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/yaml v1.2.0
)

replace google.golang.org/grpc => google.golang.org/grpc v1.29.0

replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
