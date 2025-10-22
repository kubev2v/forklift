package vsphere

import (
	"testing"

	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	. "github.com/onsi/gomega"
)

func TestHostPath(t *testing.T) {
	g := NewGomegaWithT(t)

	pb := PathBuilder{
		cache: make(map[model.Ref]*model.Base),
	}

	root := model.Folder{
		Base: model.Base{
			Name:   "Datacenters",
			Parent: model.Ref{},
			ID:     "1",
		},
	}
	rootRef := model.Ref{
		Kind: model.FolderKind,
		ID:   "1",
	}
	pb.cache[rootRef] = &root.Base

	dc := model.Datacenter{
		Base: model.Base{
			Name:   "mydc",
			Parent: rootRef,
			ID:     "2",
		},
	}
	dcRef := model.Ref{
		Kind: model.DatacenterKind,
		ID:   "2",
	}
	pb.cache[dcRef] = &dc.Base

	cluster := model.Cluster{
		Base: model.Base{
			Name:   "mycluster",
			Parent: dcRef,
			ID:     "3",
		},
	}
	clusterRef := model.Ref{
		Kind: model.ClusterKind,
		ID:   "3",
	}
	pb.cache[clusterRef] = &cluster.Base

	host := model.Host{
		Base: model.Base{
			Name:   "myhost",
			Parent: clusterRef,
			ID:     "4",
		},
	}
	hostRef := model.Ref{
		Kind: model.HostKind,
		ID:   "4",
	}
	pb.cache[hostRef] = &host.Base

	g.Expect(pb.Path(&host)).To(Equal("/mydc/mycluster/myhost"))
}

func TestHostPathNestedDatacenter(t *testing.T) {
	g := NewGomegaWithT(t)

	pb := PathBuilder{
		cache: make(map[model.Ref]*model.Base),
	}

	root := model.Folder{
		Base: model.Base{
			Name:   "Datacenters",
			Parent: model.Ref{},
			ID:     "1",
		},
	}
	rootRef := model.Ref{
		Kind: model.FolderKind,
		ID:   "1",
	}
	pb.cache[rootRef] = &root.Base

	folder := model.Folder{
		Base: model.Base{
			Name:   "myfolder",
			Parent: rootRef,
			ID:     "100",
		},
	}
	folderRef := model.Ref{
		Kind: model.FolderKind,
		ID:   "100",
	}
	pb.cache[folderRef] = &folder.Base

	dc := model.Datacenter{
		Base: model.Base{
			Name:   "mydc",
			Parent: folderRef,
			ID:     "2",
		},
	}
	dcRef := model.Ref{
		Kind: model.DatacenterKind,
		ID:   "2",
	}
	pb.cache[dcRef] = &dc.Base

	cluster := model.Cluster{
		Base: model.Base{
			Name:   "mycluster",
			Parent: dcRef,
			ID:     "3",
		},
	}
	clusterRef := model.Ref{
		Kind: model.ClusterKind,
		ID:   "3",
	}

	pb.cache[clusterRef] = &cluster.Base

	host := model.Host{
		Base: model.Base{
			Name:   "myhost",
			Parent: clusterRef,
			ID:     "4",
		},
	}
	hostRef := model.Ref{
		Kind: model.HostKind,
		ID:   "4",
	}
	pb.cache[hostRef] = &host.Base

	g.Expect(pb.Path(&host)).To(Equal("/myfolder/mydc/mycluster/myhost"))
}

func TestHostPathNestedDatacenterTwoLevels(t *testing.T) {
	g := NewGomegaWithT(t)

	pb := PathBuilder{
		cache: make(map[model.Ref]*model.Base),
	}

	root := model.Folder{
		Base: model.Base{
			Name:   "Datacenters",
			Parent: model.Ref{},
			ID:     "1",
		},
	}
	rootRef := model.Ref{
		Kind: model.FolderKind,
		ID:   "1",
	}
	pb.cache[rootRef] = &root.Base

	folder := model.Folder{
		Base: model.Base{
			Name:   "myfolder",
			Parent: rootRef,
			ID:     "100",
		},
	}
	folderRef := model.Ref{
		Kind: model.FolderKind,
		ID:   "100",
	}
	pb.cache[folderRef] = &folder.Base

	folder2 := model.Folder{
		Base: model.Base{
			Name:   "myfolder2",
			Parent: folderRef,
			ID:     "101",
		},
	}
	folder2Ref := model.Ref{
		Kind: model.FolderKind,
		ID:   "101",
	}
	pb.cache[folder2Ref] = &folder2.Base

	dc := model.Datacenter{
		Base: model.Base{
			Name:   "mydc",
			Parent: folder2Ref,
			ID:     "2",
		},
	}
	dcRef := model.Ref{
		Kind: model.DatacenterKind,
		ID:   "2",
	}
	pb.cache[dcRef] = &dc.Base

	cluster := model.Cluster{
		Base: model.Base{
			Name:   "mycluster",
			Parent: dcRef,
			ID:     "3",
		},
	}
	clusterRef := model.Ref{
		Kind: model.ClusterKind,
		ID:   "3",
	}

	pb.cache[clusterRef] = &cluster.Base

	host := model.Host{
		Base: model.Base{
			Name:   "myhost",
			Parent: clusterRef,
			ID:     "4",
		},
	}
	hostRef := model.Ref{
		Kind: model.HostKind,
		ID:   "4",
	}
	pb.cache[hostRef] = &host.Base

	g.Expect(pb.Path(&host)).To(Equal("/myfolder/myfolder2/mydc/mycluster/myhost"))
}
