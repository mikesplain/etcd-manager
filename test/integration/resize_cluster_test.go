package integration

import (
	"context"
	"testing"
	"time"

	"github.com/golang/glog"
	protoetcd "kope.io/etcd-manager/pkg/apis/etcd"
	"kope.io/etcd-manager/test/integration/harness"
)

func TestResizeCluster(t *testing.T) {
	for _, etcdVersion := range AllEtcdVersions {
		t.Run("etcdVersion="+etcdVersion, func(t *testing.T) {
			ctx := context.TODO()
			ctx, cancel := context.WithTimeout(ctx, time.Second*180)

			defer cancel()

			h := harness.NewTestHarness(t, ctx)
			h.SeedNewCluster(&protoetcd.ClusterSpec{MemberCount: 1, EtcdVersion: etcdVersion})
			defer h.Close()

			n1 := h.NewNode("127.0.0.1")
			n1.EtcdVersion = etcdVersion
			go n1.Run()

			testKey := "/testkey"

			{
				n1.WaitForListMembers(60 * time.Second)
				h.WaitForHealthy(n1)
				members1, err := n1.ListMembers(ctx)
				if err != nil {
					t.Errorf("error doing etcd ListMembers: %v", err)
				} else if len(members1) != 1 {
					t.Errorf("members was not as expected: %v", members1)
				} else {
					glog.Infof("got members from #1: %v", members1)
				}

				if err := n1.Put(ctx, testKey, "singlev2"); err != nil {
					t.Fatalf("unable to set test key: %v", err)
				}

				n1.AssertVersion(t, etcdVersion)
			}

			n2 := h.NewNode("127.0.0.2")
			n2.EtcdVersion = etcdVersion
			go n2.Run()
			n3 := h.NewNode("127.0.0.3")
			n3.EtcdVersion = etcdVersion
			go n3.Run()

			glog.Infof("expanding to cluster size 3")
			{
				h.SetClusterSpec(&protoetcd.ClusterSpec{MemberCount: 3, EtcdVersion: etcdVersion})
				h.InvalidateControlStore(n1, n2, n3)

				n1.EtcdVersion = etcdVersion
				n2.EtcdVersion = etcdVersion
				n3.EtcdVersion = etcdVersion
			}

			// Check cluster is stable (on the v3 api)
			{
				n1.WaitForListMembers(1 * time.Second)
				n2.WaitForListMembers(20 * time.Second)
				n3.WaitForListMembers(20 * time.Second)
				h.WaitForHealthy(n1, n2, n3)
				members1, err := n1.ListMembers(ctx)
				if err != nil {
					t.Errorf("error doing etcd ListMembers: %v", err)
				} else if len(members1) != 3 {
					t.Errorf("members was not as expected: %v", members1)
				} else {
					glog.Infof("got members from #1: %v", members1)
				}
			}

			// Sanity check values
			{
				v, err := n1.GetQuorum(ctx, testKey)
				if err != nil {
					t.Fatalf("error reading test key after upgrade: %v", err)
				}
				if v != "singlev2" {
					t.Fatalf("unexpected test key value after upgrade: %q", v)
				}

				n1.AssertVersion(t, etcdVersion)
				n2.AssertVersion(t, etcdVersion)
				n3.AssertVersion(t, etcdVersion)
			}

			cancel()
			h.Close()
		})
	}
}
