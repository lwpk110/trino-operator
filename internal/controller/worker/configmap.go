package worker

import (
	"context"
	"fmt"
	trinov1alpha1 "github.com/zncdata-labs/trino-operator/api/v1alpha1"
	"github.com/zncdata-labs/trino-operator/internal/common"
	"github.com/zncdata-labs/trino-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

type ConfigMapReconciler struct {
	common.ConfigurationStyleReconciler[*trinov1alpha1.TrinoCluster, *trinov1alpha1.RoleGroupSpec]
}

// NewConfigMap new a ConfigMapReconcile
func NewConfigMap(
	scheme *runtime.Scheme,
	instance *trinov1alpha1.TrinoCluster,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg *trinov1alpha1.RoleGroupSpec,
) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		ConfigurationStyleReconciler: *common.NewConfigurationStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
	}
}

// Build implements the ResourceBuilder interface
func (c *ConfigMapReconciler) Build(_ context.Context) (client.Object, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      createWorkerConfigmapName(c.Instance.GetName(), c.GroupName),
			Namespace: c.Instance.Namespace,
			Labels:    c.MergedLabels,
		},
		Data: map[string]string{
			trinov1alpha1.NodePropertiesFileName:            c.makeNodeConfigData(),
			trinov1alpha1.JvmConfigFileName:                 c.makeJvmConfigData(),
			trinov1alpha1.ConfigPropertiesFileName:          c.makeConfigPropertiesData(),
			trinov1alpha1.LogPropertiesFileName:             c.makeLogPropertiesData(),
			trinov1alpha1.ExchangeManagerPropertiesFileName: c.makeExchangeManagerPropertiesData(),
		},
	}
	return cm, nil
}

// ConfigurationOverride implement the ConfigurationOverride interface
func (c *ConfigMapReconciler) ConfigurationOverride(resource client.Object) {
	cfg := c.MergedCfg
	overrides := cfg.ConfigOverrides
	if overrides != nil {
		configMap := resource.(*corev1.ConfigMap)
		if nodeSpec := overrides.Node; nodeSpec != nil {
			if nodeProperties := util.MakePropertiesFileContent(nodeSpec); nodeProperties != "" {
				configMap.Data[trinov1alpha1.NodePropertiesFileName] = nodeProperties
			}
		}
		if jvmSpec := overrides.Jvm; jvmSpec != "" {
			configMap.Data[trinov1alpha1.JvmConfigFileName] = jvmSpec
		}
		if configSpec := overrides.Config; configSpec != nil {
			if configProperties := util.MakePropertiesFileContent(configSpec); configProperties != "" {
				configMap.Data[trinov1alpha1.ConfigPropertiesFileName] = configProperties
			}
		}
		if logSpec := overrides.Log; logSpec != nil {
			if logProperties := util.MakePropertiesFileContent(logSpec); logProperties != "" {
				configMap.Data[trinov1alpha1.LogPropertiesFileName] = logProperties
			}
		}
		if exchangeManagerSpec := overrides.ExchangeManager; exchangeManagerSpec != nil {
			if exchangeManagerProperties := util.MakePropertiesFileContent(exchangeManagerSpec); exchangeManagerProperties != "" {
				configMap.Data[trinov1alpha1.ExchangeManagerPropertiesFileName] = exchangeManagerProperties
			}
		}
	}
}

// create node.properties
const nodePropsTemplate = `node.environment=%s
node.data-dir=%s
plugin.dir=%s
`

func (c *ConfigMapReconciler) makeNodeConfigData() string {
	cfg := c.MergedCfg
	if nodeSpec := cfg.Config.NodeProperties; nodeSpec != nil {
		nodeProperties := fmt.Sprintf(nodePropsTemplate, nodeSpec.Environment, nodeSpec.DataDir, nodeSpec.PluginDir)
		return nodeProperties
	}
	return ""
}

// create jvm.config
const jvmPropsTemplate = `-server
-Xmx%s
-XX:+%s
-XX:G1HeapRegionSize=%s
-XX:+UseGCOverheadLimit
-XX:+ExplicitGCInvokesConcurrent
-XX:+HeapDumpOnOutOfMemoryError
-XX:+ExitOnOutOfMemoryError
-Djdk.attach.allowAttachSelf=true
-XX:-UseBiasedLocking
-XX:ReservedCodeCacheSize=512M
-XX:PerMethodRecompilationCutoff=10000
-XX:PerBytecodeRecompilationCutoff=10000
-Djdk.nio.maxCachedBufferSize=2000000
-XX:+UnlockDiagnosticVMOptions
-XX:+UseAESCTRIntrinsics
`

func (c *ConfigMapReconciler) makeJvmConfigData() string {
	cfg := c.MergedCfg
	if jvmSpec := cfg.Config.JvmProperties; jvmSpec != nil {
		jvmConfig := fmt.Sprintf(jvmPropsTemplate, jvmSpec.MaxHeapSize, jvmSpec.GcMethodType, jvmSpec.G1HeapRegionSize)
		return jvmConfig
	}
	return ""
}

// create config.properties
const configPropsTemplate = `coordinator=false
http-server.http.port=%s
query.max-memory=%s
query.max-memory-per-node=%s
discovery.uri=http://%s:%s
`

func (c *ConfigMapReconciler) makeConfigPropertiesData() string {
	cfg := c.MergedCfg
	if configSpec := cfg.Config.ConfigProperties; configSpec != nil {
		svc := c.getServiceSpec()
		svcPort := strconv.Itoa(int(svc.Port))
		svcName := common.CreateServiceName(c.Instance.GetName(), string(common.Coordinator), c.GroupName)
		configProperties := fmt.Sprintf(configPropsTemplate, svcPort, configSpec.QueryMaxMemory,
			configSpec.QueryMaxMemoryPerNode, svcName, svcPort)
		if configSpec.MemoryHeapHeadroomPerNode != "" {
			configProperties += "memory.heap-headroom-per-node=" + configSpec.MemoryHeapHeadroomPerNode + "\n"
		}
		if configSpec.AuthenticationType != "" {
			configProperties += "http-server.authentication.type=" + configSpec.AuthenticationType + "\n"
		}
		return configProperties
	}
	return ""
}

// create log.properties
var defaultLogProperties = `io.trino=INFO`

func (c *ConfigMapReconciler) makeLogPropertiesData() string {
	return defaultLogProperties
}

// create exchange-manager.properties
const exchangeManagerPropsTemplate = "exchange-manager.name=%s"

func (c *ConfigMapReconciler) makeExchangeManagerPropertiesData() string {
	cfg := c.MergedCfg
	if exchangeManagerSpec := cfg.Config.ExchangeManager; exchangeManagerSpec != nil {
		exchangeManagerProperties := fmt.Sprintf(exchangeManagerPropsTemplate, exchangeManagerSpec.Name) + "\n"
		if exchangeManagerSpec.Name == "filesystem" {
			exchangeManagerProperties += "exchange.base-directories=" + exchangeManagerSpec.BaseDir
		}
		return exchangeManagerProperties
	}
	return ""
}

// get serviceSpec
func (c *ConfigMapReconciler) getServiceSpec() *trinov1alpha1.ServiceSpec {
	return c.Instance.Spec.ClusterConfig.Service
}
