package operators

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cybozu-go/moco"
	"github.com/cybozu-go/moco/accessor"
	mocov1alpha1 "github.com/cybozu-go/moco/api/v1alpha1"
	"github.com/cybozu-go/well"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

const (
	host            = "localhost"
	password        = "test-password"
	primaryName     = "moco-operators-test-mysqld-primary"
	replicaName     = "moco-operators-test-mysqld-replica"
	primaryPort     = 3309
	replicaPort     = 3310
	primaryServerID = 1001
	replicaServerID = 1002
	networkName     = "moco-operators-test-net"
	namespace       = "test-namespace"
	token           = "test-token"
	mySQLVersion    = "8.0.21"
)

func TestOperators(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Operator Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	sch := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(sch)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: sch})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func getAccessorInfraCluster() (*accessor.MySQLAccessor, accessor.Infrastructure, mocov1alpha1.MySQLCluster) {
	acc := accessor.NewMySQLAccessor(&accessor.MySQLAccessorConfig{
		ConnMaxLifeTime:   30 * time.Minute,
		ConnectionTimeout: 3 * time.Second,
		ReadTimeout:       30 * time.Second,
	})
	inf := accessor.NewInfrastructure(k8sClient, acc, password, []string{host}, primaryPort)
	primaryIndex := 0
	cluster := mocov1alpha1.MySQLCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			ClusterName: "test-cluster",
			Namespace:   namespace,
			UID:         "test-uid",
		},
		Spec: mocov1alpha1.MySQLClusterSpec{
			Replicas: 1,
		},
		Status: mocov1alpha1.MySQLClusterStatus{
			CurrentPrimaryIndex: &primaryIndex,
			AgentToken:          token,
		},
	}

	return acc, inf, cluster
}

func startMySQLD(name string, port int, serverID int) error {
	ctx := context.Background()

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := well.CommandContext(ctx,
		"docker", "run", "-d", "--rm",
		"--network="+networkName,
		"--name", name,
		"-p", fmt.Sprintf("%d:%d", port, port),
		"-v", filepath.Join(wd, "..", "my.cnf")+":/etc/mysql/conf.d/my.cnf",
		"-e", "MYSQL_ROOT_PASSWORD="+password,
		"mysql:"+mySQLVersion,
		fmt.Sprintf("--port=%d", port),
		fmt.Sprintf("--server-id=%d", serverID),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopMySQLD(name string) error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "stop", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createNetwork() error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "network", "create", networkName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func removeNetwork() error {
	ctx := context.Background()
	cmd := well.CommandContext(ctx, "docker", "network", "rm", networkName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func initializeMySQL(port int) error {
	conf := mysql.NewConfig()
	conf.User = "root"
	conf.Passwd = password
	conf.Net = "tcp"
	conf.Addr = host + ":" + strconv.Itoa(port)
	conf.InterpolateParams = true

	var db *sqlx.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = sqlx.Connect("mysql", conf.FormatDSN())
		if err == nil {
			break
		}
		time.Sleep(time.Second * 3)
	}
	if err != nil {
		return err
	}

	_, err = db.Exec("CREATE USER IF NOT EXISTS ?@'%' IDENTIFIED BY ?", moco.OperatorAdminUser, password)
	if err != nil {
		return err
	}
	_, err = db.Exec("GRANT ALL ON *.* TO ?@'%' WITH GRANT OPTION", moco.OperatorAdminUser)
	if err != nil {
		return err
	}

	_, err = db.Exec("INSTALL PLUGIN rpl_semi_sync_master SONAME 'semisync_master.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'rpl_semi_sync_master' already exists" {
			return err
		}
	}
	_, err = db.Exec("INSTALL PLUGIN rpl_semi_sync_slave SONAME 'semisync_slave.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'rpl_semi_sync_slave' already exists" {
			return err
		}
	}
	_, err = db.Exec("INSTALL PLUGIN clone SONAME 'mysql_clone.so'")
	if err != nil {
		if err.Error() != "Error 1125: Function 'clone' already exists" {
			return err
		}
	}

	_, err = db.Exec(`CHANGE MASTER TO MASTER_HOST = ?, MASTER_PORT = ?, MASTER_USER = ?, MASTER_PASSWORD = ?`,
		"dummy", 3306, "dummy", "dummy")
	if err != nil {
		return err
	}
	_, err = db.Exec(`CLONE LOCAL DATA DIRECTORY = ?`, "/tmp/"+uuid.NewUUID())
	if err != nil {
		return err
	}

	return nil
}