package config

var (
	kubeContext string
)

func SetContext(kubeCtx string) {
	kubeContext = kubeCtx
}
