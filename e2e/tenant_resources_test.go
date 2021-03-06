//+build e2e

/*
Copyright 2020 Clastix Labs.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/clastix/capsule/api/v1alpha1"
)

var _ = Describe("creating namespaces within a Tenant with resources", func() {
	tnt := &v1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tenant-resources",
		},
		Spec: v1alpha1.TenantSpec{
			Owner: v1alpha1.OwnerSpec{
				Name: "john",
				Kind: "User",
			},
			LimitRanges: []corev1.LimitRangeSpec{
				{
					Limits: []corev1.LimitRangeItem{
						{
							Type: corev1.LimitTypePod,
							Min: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("5Mi"),
							},
							Max: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						{
							Type: corev1.LimitTypeContainer,
							Default: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
							DefaultRequest: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("10Mi"),
							},
							Min: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("5Mi"),
							},
							Max: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						{
							Type: corev1.LimitTypePersistentVolumeClaim,
							Min: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
							Max: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				},
			},
			NetworkPolicies: []networkingv1.NetworkPolicySpec{
				{
					Ingress: []networkingv1.NetworkPolicyIngressRule{
						{
							From: []networkingv1.NetworkPolicyPeer{
								{
									NamespaceSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"capsule.clastix.io/tenant": "tenant-resources",
										},
									},
								},
								{
									PodSelector: &metav1.LabelSelector{},
								},
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR: "192.168.0.0/12",
									},
								},
							},
						},
					},
					Egress: []networkingv1.NetworkPolicyEgressRule{
						{
							To: []networkingv1.NetworkPolicyPeer{
								{
									IPBlock: &networkingv1.IPBlock{
										CIDR: "0.0.0.0/0",
										Except: []string{
											"192.168.0.0/12",
										},
									},
								},
							},
						},
					},
					PodSelector: metav1.LabelSelector{},
					PolicyTypes: []networkingv1.PolicyType{
						networkingv1.PolicyTypeIngress,
						networkingv1.PolicyTypeEgress,
					},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			ResourceQuota: []corev1.ResourceQuotaSpec{
				{
					Hard: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceLimitsCPU:      resource.MustParse("8"),
						corev1.ResourceLimitsMemory:   resource.MustParse("16Gi"),
						corev1.ResourceRequestsCPU:    resource.MustParse("8"),
						corev1.ResourceRequestsMemory: resource.MustParse("16Gi"),
					},
					Scopes: []corev1.ResourceQuotaScope{
						corev1.ResourceQuotaScopeNotTerminating,
					},
				},
				{
					Hard: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourcePods: resource.MustParse("10"),
					},
				},
				{
					Hard: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceRequestsStorage: resource.MustParse("100Gi"),
					},
				},
			},
		},
	}
	nsl := []string{"bim", "bum", "bam"}
	JustBeforeEach(func() {
		EventuallyCreation(func() error {
			return k8sClient.Create(context.TODO(), tnt)
		}).Should(Succeed())
		By("creating the Namespaces", func() {
			for _, i := range nsl {
				ns := NewNamespace(i)
				NamespaceCreation(ns, tnt, defaultTimeoutInterval).Should(Succeed())
				TenantNamespaceList(tnt, podRecreationTimeoutInterval).Should(ContainElement(ns.GetName()))
			}
		})
	})
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.TODO(), tnt)).Should(Succeed())
	})
	It("should contains all replicated resources", func() {
		for _, name := range nsl {
			By("checking Limit Range", func() {
				for i, s := range tnt.Spec.LimitRanges {
					n := fmt.Sprintf("capsule-%s-%d", tnt.GetName(), i)
					lr := &corev1.LimitRange{}
					Eventually(func() error {
						return k8sClient.Get(context.TODO(), types.NamespacedName{Name: n, Namespace: name}, lr)
					}, defaultTimeoutInterval, defaultPollInterval).Should(Succeed())
					Expect(lr.Spec).Should(Equal(s))
				}
			})
			By("checking Network Policy", func() {
				for i, s := range tnt.Spec.NetworkPolicies {
					n := fmt.Sprintf("capsule-%s-%d", tnt.GetName(), i)
					np := &networkingv1.NetworkPolicy{}
					Eventually(func() error {
						return k8sClient.Get(context.TODO(), types.NamespacedName{Name: n, Namespace: name}, np)
					}, defaultTimeoutInterval, defaultPollInterval).Should(Succeed())
					Expect(np.Spec).Should(Equal(s))
				}
			})
			By("checking the Namespace scheduler annotation", func() {
				var selector []string
				for k, v := range tnt.Spec.NodeSelector {
					selector = append(selector, fmt.Sprintf("%s=%s", k, v))
				}
				Eventually(func() string {
					ns := &corev1.Namespace{}
					Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: name}, ns)).Should(Succeed())
					return ns.GetAnnotations()["scheduler.alpha.kubernetes.io/node-selector"]
				}, defaultTimeoutInterval, defaultPollInterval).Should(Equal(strings.Join(selector, ",")))
			})
			By("checking the Resource Quota", func() {
				for i, s := range tnt.Spec.ResourceQuota {
					n := fmt.Sprintf("capsule-%s-%d", tnt.GetName(), i)
					rq := &corev1.ResourceQuota{}
					Eventually(func() error {
						return k8sClient.Get(context.TODO(), types.NamespacedName{Name: n, Namespace: name}, rq)
					}, defaultTimeoutInterval, defaultPollInterval).Should(Succeed())
					Expect(rq.Spec).Should(Equal(s))
				}
			})
		}
	})
})
