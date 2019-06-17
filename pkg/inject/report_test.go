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
	t.Run("with 'linkerd.io/inject: enabled", func(t *testing.T) {
		var testCases = []struct {
			podSpec             *corev1.PodSpec
			nsAnnotations       map[string]string
			unsupportedResource bool
			injectable          bool
		}{
			{
				podSpec:    &corev1.PodSpec{HostNetwork: false},
				injectable: true,
			},
			{
				podSpec:    &corev1.PodSpec{HostNetwork: true},
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
				injectable: false,
			},
			{
				unsupportedResource: true,
				podSpec:             &corev1.PodSpec{},
				injectable:          false,
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
				resourceConfig.pod.meta = &metav1.ObjectMeta{
					Annotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
				}

				report := newReport(resourceConfig)
				report.UnsupportedResource = testCase.unsupportedResource

				if actual := report.Injectable(); testCase.injectable != actual {
					t.Errorf("Expected %t. Actual %t", testCase.injectable, actual)
				}
			})
		}
	})
}

func TestInjectDisabled(t *testing.T) {
	t.Run("CLI origin", func(t *testing.T) {
		var testCases = []struct {
			inject   string
			expected bool
		}{
			{k8s.ProxyInjectEnabled, false},
			{k8s.ProxyInjectDisabled, true},
		}

		config := NewResourceConfig(&config.All{}, OriginCLI)
		config.pod.spec = &corev1.PodSpec{}
		for i, testCase := range testCases {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				testCase := testCase
				t.Run(fmt.Sprintf("with 'linkerd.io/inject: %s", testCase.inject), func(t *testing.T) {
					config.pod.meta = &metav1.ObjectMeta{
						Annotations: map[string]string{
							k8s.ProxyInjectAnnotation: testCase.inject,
						},
					}

					report := newReport(config)
					if report.injectDisabled(config) != testCase.expected {
						t.Errorf("expect injectDisabled() to return %t with linkerd.io/inject: %s", testCase.expected, testCase.inject)
					}
				})
			})
		}
	})

	t.Run("webhook origin", func(t *testing.T) {
		t.Run("with 'linkerd.io/inject: disabled", func(t *testing.T) {
			config := NewResourceConfig(&config.All{}, OriginCLI)
			config.pod.spec = &corev1.PodSpec{}
			config.pod.meta = &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: "disabled",
				},
			}

			report := newReport(config)
			if !report.injectDisabled(config) {
				t.Error("Expected injectDisabled() to return true with 'linkerd.io/inject: disabled'")
			}
		})

		t.Run("with 'linkerd.io/inject: enabled", func(t *testing.T) {
			config := NewResourceConfig(&config.All{
				Global: &config.Global{},
			}, OriginWebhook)
			config.pod.spec = &corev1.PodSpec{}
			config.pod.meta = &metav1.ObjectMeta{
				Annotations: map[string]string{
					k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
				},
			}

			t.Run("managed-by annotation matches control plane namespace", func(t *testing.T) {
				var testCases = []struct {
					controlPlaneNS    string // control plane namespace that the proxy injector belongs to
					workloadManagedBy string // value of the 'managed-by' annotation on the workload
				}{
					{
						controlPlaneNS:    k8s.ControlPlaneDefaultNS,
						workloadManagedBy: "", // expect default namespace to be used
					},
					{
						controlPlaneNS:    k8s.ControlPlaneDefaultNS,
						workloadManagedBy: k8s.ControlPlaneDefaultNS,
					},
					{
						controlPlaneNS:    "linkerd-dev",
						workloadManagedBy: "linkerd-dev",
					},
				}

				for i, testCase := range testCases {
					t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
						testCase := testCase
						config.configs.GetGlobal().LinkerdNamespace = testCase.controlPlaneNS
						config.pod.meta.Annotations[k8s.ProxyManagedByAnnotation] = testCase.workloadManagedBy

						report := newReport(config)
						if report.injectDisabled(config) {
							t.Error("expect injectDisabled() to return false as workload is injectable")
						}
					})
				}
			})

			t.Run("managed-by annotation doesn't match control plane namespace", func(t *testing.T) {
				var testCases = []struct {
					controlPlaneNS    string // control plane namespace that the proxy injector belongs to
					workloadManagedBy string // value of the 'managed-by' annotation on the workload
				}{
					{
						controlPlaneNS:    k8s.ControlPlaneDefaultNS,
						workloadManagedBy: "linkerd-dev",
					},
					{
						controlPlaneNS:    "linkerd-dev",
						workloadManagedBy: k8s.ControlPlaneDefaultNS,
					},
				}

				for i, testCase := range testCases {
					t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
						testCase := testCase
						config.configs.GetGlobal().LinkerdNamespace = testCase.controlPlaneNS
						config.pod.meta.Annotations[k8s.ProxyManagedByAnnotation] = testCase.workloadManagedBy

						report := newReport(config)
						if !report.injectDisabled(config) {
							t.Error("expect injectDisabled() to return true as workload isn't injectable")
						}
					})
				}
			})
		})
	})
}

func TestDisableByAnnotation(t *testing.T) {
	t.Run("webhook origin", func(t *testing.T) {
		t.Run("disable by pod annotation", func(t *testing.T) {
			var testCases = []struct {
				podMeta  *metav1.ObjectMeta
				expected bool
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
							k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
						},
					},
					expected: true,
				},
				{
					podMeta:  &metav1.ObjectMeta{},
					expected: true,
				},
			}

			for i, testCase := range testCases {
				testCase := testCase
				t.Run(fmt.Sprintf("test case #%d", i), func(t *testing.T) {
					resourceConfig := &ResourceConfig{origin: OriginWebhook}
					resourceConfig.pod.meta = testCase.podMeta

					report := newReport(resourceConfig)
					if actual := report.disableByAnnotation(resourceConfig); testCase.expected != actual {
						t.Errorf("Expected %t. Actual %t", testCase.expected, actual)
					}
				})
			}
		})

		t.Run("disable by namespace annotation", func(t *testing.T) {
			var testCases = []struct {
				nsAnnotations map[string]string
				expected      bool
			}{
				{
					nsAnnotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
					},
					expected: true,
				},
				{
					nsAnnotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
					expected: false,
				},
				{
					nsAnnotations: map[string]string{},
					expected:      true,
				},
			}

			for i, testCase := range testCases {
				testCase := testCase
				t.Run(fmt.Sprintf("test case #%d", i), func(t *testing.T) {
					resourceConfig := &ResourceConfig{origin: OriginWebhook}
					resourceConfig.pod.meta = &metav1.ObjectMeta{}
					resourceConfig.WithNsAnnotations(testCase.nsAnnotations)

					report := newReport(resourceConfig)
					if actual := report.disableByAnnotation(resourceConfig); testCase.expected != actual {
						t.Errorf("Expected %t. Actual %t", testCase.expected, actual)
					}
				})
			}
		})

		t.Run("pod annotation precedes namespace annotation", func(t *testing.T) {
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
					nsAnnotations: map[string]string{
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectDisabled,
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
						k8s.ProxyInjectAnnotation: k8s.ProxyInjectEnabled,
					},
					expected: true,
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
		expected          string // expected result of report.targetControlPlane()
	}{
		{
			controlPlaneNS:    k8s.ControlPlaneDefaultNS,
			workloadManagedBy: "",
			expected:          k8s.ControlPlaneDefaultNS,
		},
		{
			controlPlaneNS:    k8s.ControlPlaneDefaultNS,
			workloadManagedBy: k8s.ControlPlaneDefaultNS,
			expected:          k8s.ControlPlaneDefaultNS,
		},
		{
			controlPlaneNS:    "linkerd-dev",
			workloadManagedBy: "linkerd-dev",
			expected:          "linkerd-dev",
		},
		{
			controlPlaneNS:    "linkerd-dev",
			workloadManagedBy: k8s.ControlPlaneDefaultNS,
			expected:          k8s.ControlPlaneDefaultNS,
		},
		{
			controlPlaneNS:    k8s.ControlPlaneDefaultNS,
			workloadManagedBy: "linkerd-dev",
			expected:          "linkerd-dev",
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			testCase := testCase
			configs := &config.All{
				Global: &config.Global{LinkerdNamespace: testCase.controlPlaneNS},
			}
			config := NewResourceConfig(configs, OriginUnknown)
			config.pod.spec = &corev1.PodSpec{}
			config.pod.meta = &metav1.ObjectMeta{
				Annotations: map[string]string{},
			}

			t.Run("pod level annotation", func(t *testing.T) {
				config.pod.meta.Annotations[k8s.ProxyManagedByAnnotation] = testCase.workloadManagedBy
				report := newReport(config)
				if actual := report.targetControlPlane(config); actual != testCase.expected {
					t.Errorf("Namespace mismatch. Expected: %s. Actual: %s", testCase.expected, actual)
				}
			})

			t.Run("namespace level annotation", func(t *testing.T) {
				config.nsAnnotations = map[string]string{
					k8s.ProxyManagedByAnnotation: testCase.workloadManagedBy,
				}

				report := newReport(config)
				if actual := report.targetControlPlane(config); actual != testCase.expected {
					t.Errorf("Namespace mismatch. Expected: %s. Actual: %s", testCase.expected, actual)
				}
			})
		})
	}

	t.Run("pod annotation precedes namespace annotation", func(t *testing.T) {
		var testCases = []struct {
			controlPlaneNS    string // control plane namespace that the proxy injector belongs to
			podLevelManagedBy string // value of the 'managed-by' annotation at the pod level
			nsLevelManagedBy  string // value of the 'managed-by' annotation at the namespace level
			expected          string // expected result of report.targetControlPlane()
		}{
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: "",
				nsLevelManagedBy:  "",
				expected:          k8s.ControlPlaneDefaultNS,
			},
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: k8s.ControlPlaneDefaultNS,
				nsLevelManagedBy:  "",
				expected:          k8s.ControlPlaneDefaultNS,
			},
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: "",
				nsLevelManagedBy:  k8s.ControlPlaneDefaultNS,
				expected:          k8s.ControlPlaneDefaultNS,
			},
			{
				controlPlaneNS:    "linkerd-dev",
				podLevelManagedBy: "linkerd-dev",
				nsLevelManagedBy:  k8s.ControlPlaneDefaultNS,
				expected:          "linkerd-dev",
			},
			{
				controlPlaneNS:    k8s.ControlPlaneDefaultNS,
				podLevelManagedBy: "linkerd-dev1",
				nsLevelManagedBy:  "linkerd-dev2",
				expected:          "linkerd-dev1",
			},
		}

		for i, testCase := range testCases {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
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
				if actual := report.targetControlPlane(config); actual != testCase.expected {
					t.Errorf("Namespace mismatch. Expected: %s. Actual: %s", testCase.expected, actual)
				}
			})
		}
	})
}
