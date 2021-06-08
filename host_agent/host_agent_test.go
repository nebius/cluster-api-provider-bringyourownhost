package main

import (
	"context"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	infrastructurev1alpha4 "github.com/vmware-tanzu/cluster-api-provider-byoh/api/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/patch"
)

var _ = Describe("HostAgent", func() {
	var (
		ns      = &corev1.Namespace{}
		session *gexec.Session
		err     error
	)

	BeforeEach(func() {
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "testns-" + rand.String(5)},
		}

		err = k8sClient.Create(context.TODO(), ns)
		Expect(err).NotTo(HaveOccurred(), "failed to create test namespace")

	})

	AfterEach(func() {

		err = k8sClient.Delete(context.TODO(), ns)
		Expect(err).NotTo(HaveOccurred(), "failed to delete test namespace")
	})

	It("should error out if the host already exists", func() {
		ByoHost := &infrastructurev1alpha4.ByoHost{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ByoHost",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      hostName,
				Namespace: ns.Name,
			},
			Spec: infrastructurev1alpha4.ByoHostSpec{},
		}
		err = k8sClient.Create(context.TODO(), ByoHost)

		command := exec.Command(pathToHostAgentBinary, "--kubeconfig", kubeconfigFile.Name(), "--namespace", ns.Name)
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		Eventually(session).Should(gexec.Exit(255))
	})

	It("should return an error when invalid kubeconfig is passed in", func() {
		command := exec.Command(pathToHostAgentBinary, "--kubeconfig", "non-existent-path")
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(255))
	})

	Context("When the host agent is able to connect to API Server", func() {
		BeforeEach(func() {
			command := exec.Command(pathToHostAgentBinary, "--kubeconfig", kubeconfigFile.Name(), "--namespace", ns.Name)
			session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session.Terminate().Wait()
		})

		It("should register the BYOHost with the management cluster", func() {

			byoHostLookupKey := types.NamespacedName{Name: "jaime.com", Namespace: ns.Name}
			Eventually(func() *infrastructurev1alpha4.ByoHost {
				createdByoHost := &infrastructurev1alpha4.ByoHost{}
				err := k8sClient.Get(context.TODO(), byoHostLookupKey, createdByoHost)
				if err != nil {
					return nil
				}
				return createdByoHost
			}).ShouldNot(BeNil())
		})

		It("should bootstrap the node when MachineRef is set", func() {

			byoHost := &infrastructurev1alpha4.ByoHost{}
			byoHostLookupKey := types.NamespacedName{Name: "jaime.com", Namespace: ns.Name}
			Eventually(func() bool {
				err = k8sClient.Get(context.TODO(), byoHostLookupKey, byoHost)
				return err == nil
			}).ShouldNot(BeFalse())

			secret := createSecretCRD("bootstrap-secret-1", "ZWNobyAiak1lIGlzIH5hd2Vzb21lIg==", ns.Name)
			machine := createClusterAPIMachine(&secret.Name, "test-machine", ns.Name)
			byoMachine := createBYOMachineCRD("test-byomachine", ns.Name, machine)

			helper, err := patch.NewHelper(byoHost, k8sClient)
			Expect(err).ToNot(HaveOccurred())

			byoHost.Status.MachineRef = &corev1.ObjectReference{
				Kind:       "ByoMachine",
				Namespace:  ns.Name,
				Name:       byoMachine.Name,
				UID:        byoMachine.UID,
				APIVersion: byoHost.APIVersion,
			}
			err = helper.Patch(context.TODO(), byoHost)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() corev1.ConditionStatus {
				createdByoHost := &infrastructurev1alpha4.ByoHost{}
				err := k8sClient.Get(context.TODO(), byoHostLookupKey, createdByoHost)
				if err != nil {
					return corev1.ConditionFalse
				}

				for _, condition := range createdByoHost.Status.Conditions {
					if condition.Type == infrastructurev1alpha4.K8sComponentsInstalledCondition {
						return condition.Status
					}
				}
				return corev1.ConditionFalse
			}).Should(Equal(corev1.ConditionTrue))

			Eventually(session.Out).Should(gbytes.Say("jMe is ~awesome"))

		})
	})

})

func createSecretCRD(bootstrapSecretName, stringDataValue, namespace string) *corev1.Secret {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapSecretName,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"value": stringDataValue,
		},
		Type: "cluster.x-k8s.io/secret",
	}

	err = k8sClient.Create(context.TODO(), secret)
	Expect(err).ToNot(HaveOccurred())
	return secret
}

func createClusterAPIMachine(bootstrapSecret *string, machineName, namespace string) *clusterv1.Machine {
	machine := &clusterv1.Machine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Machine",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: namespace,
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: bootstrapSecret,
			},
			ClusterName: "default-test-cluster",
		},
	}

	err = k8sClient.Create(context.TODO(), machine)
	Expect(err).ToNot(HaveOccurred())
	return machine
}

func createBYOMachineCRD(byoMachineName, namespace string, machine *clusterv1.Machine) *infrastructurev1alpha4.ByoMachine {
	byoMachine := &infrastructurev1alpha4.ByoMachine{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ByoMachine",
			APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha4",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      byoMachineName,
			Namespace: namespace,

			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Machine",
					Name:       machine.Name,
					APIVersion: "v1",
					UID:        machine.UID,
				},
			},
		},
		Spec: infrastructurev1alpha4.ByoMachineSpec{},
	}

	err = k8sClient.Create(context.TODO(), byoMachine)
	Expect(err).ToNot(HaveOccurred())

	return byoMachine
}