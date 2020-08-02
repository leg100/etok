package command

import (
	"github.com/operator-framework/operator-sdk/pkg/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Interface interface {
	runtime.Object
	schema.ObjectKind
	metav1.Object

	GetConditions() *status.Conditions
	SetConditions(status.Conditions)
	GetArgs() []string
	SetArgs([]string)
	GetTimeoutClient() string
	SetTimeoutClient(string)
	GetTimeoutQueue() string
	SetTimeoutQueue(string)
	GetDebug() bool
	SetDebug(bool)
	GetConfigMap() string
	SetConfigMap(string)
	GetConfigMapKey() string
	SetConfigMapKey(string)
	GetWorkspace() string
	SetWorkspace(string)
}
