package operators

import (
	"context"
	"fmt"

	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco/accessor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Set clone donor list", func() {

	ctx := context.Background()

	BeforeEach(func() {
		err := startMySQLD(mysqldName1, mysqldPort1, mysqldServerID1)
		Expect(err).ShouldNot(HaveOccurred())
		err = startMySQLD(mysqldName2, mysqldPort2, mysqldServerID2)
		Expect(err).ShouldNot(HaveOccurred())

		err = initializeMySQL(mysqldPort1)
		Expect(err).ShouldNot(HaveOccurred())
		err = initializeMySQL(mysqldPort2)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		stopMySQLD(mysqldName1)
		stopMySQLD(mysqldName2)
	})

	logger := ctrl.Log.WithName("operators-test")

	It("should configure replication", func() {
		_, infra, cluster := getAccessorInfraCluster()

		op := setCloneDonorListOp{}

		err := op.Run(ctx, infra, &cluster, nil)
		Expect(err).ShouldNot(HaveOccurred())

		host := moco.GetHost(&cluster, *cluster.Status.CurrentPrimaryIndex)
		hostWithPort := fmt.Sprintf("%s:%d", host, moco.MySQLAdminPort)
		status := accessor.GetMySQLClusterStatus(ctx, logger, infra, &cluster)
		Expect(status.InstanceStatus).Should(HaveLen(2))
		Expect(status.InstanceStatus[0].GlobalVariablesStatus.CloneValidDonorList.Valid).Should(BeTrue())
		Expect(status.InstanceStatus[0].GlobalVariablesStatus.CloneValidDonorList.String).Should(Equal(hostWithPort))
		Expect(status.InstanceStatus[1].GlobalVariablesStatus.CloneValidDonorList.Valid).Should(BeTrue())
		Expect(status.InstanceStatus[1].GlobalVariablesStatus.CloneValidDonorList.String).Should(Equal(hostWithPort))
	})
})