package inject

import (
	"fmt"
	"testing"

	"github.com/linkerd/linkerd2/controller/gen/config"
	"github.com/linkerd/linkerd2/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInjectable(t *testing.T) {
	var testCases = []struct {
		podSpec             *corev1.PodSpec
		podMeta             *metav1.ObjectMeta
		nsAnnotations       map[string]string
		unsupportedResource bool
		injectable          bool
	}{
		{
			podSpec: &corev1.PodSpec{HostNetwork: false},
			podMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
			},
			injectable: true,
		},
		{
			podSpec: &corev1.PodSpec{HostNetwork: true},
			podMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
			},
			injectable: false,
		},
		{
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  k8s.ProxyContainerName,
						Image: "gcr.io/linkerd-io/proxy:",
					},
				},
			},
			podMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
			},
			injectable: false,
		},
		{
			podSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:  k8s.InitContainerName,
						Image: "gcr.io/linkerd-io/proxy-init:",
					},
				},
			},
			podMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
			},
			injectable: false,
		},
		{
			unsupportedResource: true,
			podSpec:             &corev1.PodSpec{},
			podMeta: &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
			},
			injectable: false,
		},
	}

	for i, testCase := range testCases {
		testCase := testCase
		t.Run(fmt.Sprintf("test case #%d", i), func(t *testing.T) {
			resourceConfig := &ResourceConfig{
				configs: &config.All{
					Global: &config.Global{LinkerdNamespace: k8s.ControlPlaneDefaultNS},
				},
			}
			resourceConfig.WithNsAnnotations(testCase.nsAnnotations)
			resourceConfig.pod.spec = testCase.podSpec
			resourceConfig.pod.meta = testCase.podMeta

			report := newReport(resourceConfig)
			report.UnsupportedResource = testCase.unsupportedResource

			if actual := report.Injectable(); testCase.injectable != actual {
				t.Errorf("Expected %t. Actual %t", testCase.injectable, actual)
			}
		})
	}
}

func TestDisableByAnnotation(t *testing.T) {
	t.Run("webhook origin", func(t *testing.T) {
		var testCases = []struct {
			podMeta       *metav1.ObjectMeta
			nsAnnotations map[string]string
			expected      bool
		}{
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
				},
				expected: false,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
				},
				nsAnnotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
				expected: false,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
				},
				nsAnnotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
				},
				expected: false,
			},
			{
				podMeta: &metav1.ObjectMeta{},
				nsAnnotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
				expected: false,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
					},
				},
				nsAnnotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
				},
				expected: true,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
					},
				},
				nsAnnotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
				expected: true,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
					},
				},
				nsAnnotations: map[string]string{},
				expected:      true,
			},
			{
				podMeta: &metav1.ObjectMeta{},
				nsAnnotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
				},
				expected: true,
			},
			{
				podMeta:       &metav1.ObjectMeta{},
				nsAnnotations: map[string]string{},
				expected:      true,
			},
		}

		for i, testCase := range testCases {
			testCase := testCase
			t.Run(fmt.Sprintf("test case #%d", i), func(t *testing.T) {
				resourceConfig := &ResourceConfig{origin: OriginWebhook}
				resourceConfig.WithNsAnnotations(testCase.nsAnnotations)
				resourceConfig.pod.meta = testCase.podMeta

				report := newReport(resourceConfig)
				if actual := report.disableByAnnotation(resourceConfig); testCase.expected != actual {
					t.Errorf("Expected %t. Actual %t", testCase.expected, actual)
				}
			})
		}
	})

	t.Run("CLI origin", func(t *testing.T) {
		var testCases = []struct {
			podMeta  *metav1.ObjectMeta
			expected bool
		}{
			{
				podMeta:  &metav1.ObjectMeta{},
				expected: false,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
				},
				expected: false,
			},
			{
				podMeta: &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
					},
				},
				expected: true,
			},
		}

		for i, testCase := range testCases {
			testCase := testCase
			t.Run(fmt.Sprintf("test case #%d", i), func(t *testing.T) {
				resourceConfig := &ResourceConfig{origin: OriginCLI}
				resourceConfig.pod.meta = testCase.podMeta

				report := newReport(resourceConfig)
				if actual := report.disableByAnnotation(resourceConfig); testCase.expected != actual {
					t.Errorf("Expected %t. Actual %t", testCase.expected, actual)
				}
			})
		}
	})
}

func TestTargetControlPlane(t *testing.T) {
	var testCases = []struct {
		controlPlaneNS    string // control plane namespace that the proxy injector belongs to
		workloadManagedBy string // value of the 'managed-by' annotation on the workload
		expectedNamespace string // expected result of report.targetControlPlane()
		expectedManagedBy bool   // true if controlPlaneNS ==  workloadManagedBy
	}{
		{
			controlPlaneNS:    k8s.ControlPlaneDefaultNS,
			workloadManagedBy: "",
			expectedNamespace: k8s.ControlPlaneDefaultNS,
			expectedManagedBy: true,
		},
		{
			controlPlaneNS:    k8s.ControlPlaneDefaultNS,
			workloadManagedBy: k8s.ControlPlaneDefaultNS,
			expectedNamespace: k8s.ControlPlaneDefaultNS,
			expectedManagedBy: true,
		},
		{
			controlPlaneNS:    "linkerd-dev",
			workloadManagedBy: "linkerd-dev",
			expectedNamespace: "linkerd-dev",
			expectedManagedBy: true,
		},
		{
			controlPlaneNS:    "linkerd-dev",
			workloadManagedBy: k8s.ControlPlaneDefaultNS,
			expectedNamespace: k8s.ControlPlaneDefaultNS,
			expectedManagedBy: false,
		},
		{
			controlPlaneNS:    k8s.ControlPlaneDefaultNS,
			workloadManagedBy: "linkerd-dev",
			expectedNamespace: "linkerd-dev",
			expectedManagedBy: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		configs := &config.All{
			Global: &config.Global{LinkerdNamespace: testCase.controlPlaneNS},
		}

		t.Run("pod level annotation", func(t *testing.T) {
			config := NewResourceConfig(configs, OriginUnknown)
			config.pod.spec = &corev1.PodSpec{}
			config.pod.meta = &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyManagedByAnnotation: testCase.workloadManagedBy,
				},
			}

			report := newReport(config)
			if actual := report.targetControlPlane(config); actual != testCase.expectedNamespace {
				t.Errorf("Namespace mismatch. Expected: %s. Actual: %s", testCase.expectedNamespace, actual)
			}

			if report.ManagedBy != testCase.expectedManagedBy {
				t.Errorf("Mismatch in 'managed by' values. Expected: %t. Actual: %t", testCase.expectedManagedBy, report.ManagedBy)
			}
		})

		t.Run("namespace level annotation", func(t *testing.T) {
			config := NewResourceConfig(configs, OriginUnknown)
			config.pod.spec = &corev1.PodSpec{}
			config.pod.meta = &metav1.ObjectMeta{}
			config.nsAnnotations = map[string]string{
				k8s.ProxyManagedByAnnotation: testCase.workloadManagedBy,
			}

			report := newReport(config)
			if actual := report.targetControlPlane(config); actual != testCase.expectedNamespace {
				t.Errorf("Namespace mismatch. Expected: %s. Actual: %s", testCase.expectedNamespace, actual)
			}

			if report.ManagedBy != testCase.expectedManagedBy {
				t.Errorf("Mismatch in 'managed by' values. Expected: %t. Actual: %t", testCase.expectedManagedBy, report.ManagedBy)
			}
		})
	}

	t.Run("pod annotation precedes namespace annotation", func(t *testing.T) {
		var testCases = []struct {
			controlPlaneNS    string // control plane namespace that the proxy injector belongs to
			podLevelManagedBy string // value of the 'managed-by' annotation at the pod level
			nsLevelManagedBy  string // value of the 'managed-by' annotation at the namespace level
			expectedNamespace string // expected result of report.targetControlPlane()
			expectedManagedBy bool   // true if controlPlaneNS ==  workloadManagedBy
		}{
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: "",
				nsLevelManagedBy:  "",
				expectedNamespace: k8s.ControlPlaneDefaultNS,
				expectedManagedBy: true,
			},
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: k8s.ControlPlaneDefaultNS,
				nsLevelManagedBy:  "",
				expectedNamespace: k8s.ControlPlaneDefaultNS,
				expectedManagedBy: true,
			},
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: "",
				nsLevelManagedBy:  k8s.ControlPlaneDefaultNS,
				expectedNamespace: k8s.ControlPlaneDefaultNS,
				expectedManagedBy: true,
			},
			{
				controlPlaneNS:    "linkerd-dev",
				podLevelManagedBy: "linkerd-dev",
				nsLevelManagedBy:  k8s.ControlPlaneDefaultNS,
				expectedNamespace: "linkerd-dev",
				expectedManagedBy: true,
			},
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: "linkerd-dev1",
				nsLevelManagedBy:  "linkerd-dev2",
				expectedNamespace: "linkerd-dev1",
				expectedManagedBy: false,
			},
		}
		for _, testCase := range testCases {
			testCase := testCase
			configs := &config.All{
				Global: &config.Global{LinkerdNamespace: testCase.controlPlaneNS},
			}

			config := NewResourceConfig(configs, OriginUnknown)
			config.pod.spec = &corev1.PodSpec{}
			config.pod.meta = &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyManagedByAnnotation: testCase.podLevelManagedBy,
				},
			}
			config.nsAnnotations = map[string]string{
				k8s.ProxyManagedByAnnotation: testCase.nsLevelManagedBy,
			}

			report := newReport(config)
			if actual := report.targetControlPlane(config); actual != testCase.expectedNamespace {
				t.Errorf("Namespace mismatch. Expected: %s. Actual: %s", testCase.expectedNamespace, actual)
			}

			if report.ManagedBy != testCase.expectedManagedBy {
				t.Errorf("Mismatch in 'managed by' values. Expected: %t. Actual: %t", testCase.expectedManagedBy, report.ManagedBy)
			}
		}
	})
}
